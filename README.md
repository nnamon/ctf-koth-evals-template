# ctf-koth-evals-template

A starter template for building evals platforms for **King-of-the-Hill (KOTH) style CTF challenges**.

## What this is for

KOTH-style CTF challenges typically work like this:

- Each challenge ships with an **engine** (the environment, simulator, or scoring harness).
- Players submit a **solution** (code, agent, config, prompt — challenge-dependent).
- The engine runs the submission and produces a **score** based on performance.
- Submissions are ranked on a **leaderboard**.

Every time we run one of these, we end up rebuilding the same scaffolding: submission intake, sandboxed execution, scoring, leaderboard, auth. This repo is meant to be the reusable base so the next challenge only has to plug in the engine + scoring logic.

## Goals

- Provide a working web app out of the box: submission upload, run history, leaderboard.
- Keep the **challenge-specific engine** as a clean extension point — swap it per challenge without touching the platform.
- Standardize the execution + scoring lifecycle so results are comparable and reproducible.

## Stack (planned)

- **Backend:** Go — solid subprocess handling (`os/exec` + `context.Context` for timeouts/cancellation), cheap concurrency for a bounded worker pool, single-binary deploys.
- **Frontend:** Separate React + Vite SPA in `web/`, wired to the API via fetch + session-stored basic-auth. Routes: `/login`, `/` (leaderboard with suite picker), `/suites/new`, `/suites/:id`, `/submit`, `/submissions`, `/submissions/:id`. Aesthetic from [agent-aesthetics](https://github.com/nnamon/agent-aesthetics) (`soft-brutalist-document`, **M2 Magenta** palette). Light is the default; `palette-m2d` is applied when the system prefers dark, with a manual toggle.
- **Persistence:** [Ent](https://entgo.io) — schema-as-code, codegen produces a type-safe client. Drivers cover SQLite (local dev) and Postgres (prod) with no schema changes.
- **Deployment:** App ships as a Docker image. Engines run as subprocesses *inside* the app container — no per-run containerization.
- **Run model:** Asynchronous, DB-backed queue. Pending runs are rows in the `runs` table; the server's `/internal/runs/claim` handler atomically claims one row per call (`BEGIN`, query pending, `UPDATE`, `COMMIT`) behind a server-side mutex so SQLite doesn't `SQLITE_BUSY` under concurrent workers. Adding throughput = running more worker processes. See [Scaling](#scaling).

## Core concepts

- **Challenge** — an engine + wrapper bundle. The testing apparatus for a given problem; fixed for the duration of a CTF.
- **Suite** — a test configuration layered over a challenge: which engine to use, what parameters, how many runs, which seeds, timeouts, and scoring/aggregation. Suites are first-class so the configuration in play can change during a competition (e.g. qualifier → finals) without changing the engine itself.
- **Submission** — a player artifact uploaded for evaluation.
- **Run** — one execution of a submission against a suite at a specific seed. A submission against a suite typically produces many runs, which the suite's scoring config aggregates into a final score.

A suite roughly contains:

```
suite:
  challenge_ref:     # which challenge to use
  parameters:        # opaque config passed through to the wrapper
  runs:              # N — how many executions per submission
  seeds:             # explicit fixed list (same seeds for every submission → directly comparable)
  timeout_per_run:
  scoring:           # aggregation strategy + tiebreakers
```

### Suite rules

- **Multiple suites coexist.** Each suite has its own leaderboard. No cross-suite aggregate ranking.
- **Player picks the suite at submit time.** The UI offers sensible defaults (most recent / admin-featured).
- **Re-evaluation is supported, manually.** Admins can run an existing submission against a different suite; this just enqueues new runs.
- **Fixed seeds.** Each suite carries an explicit seed list; every submission against the suite runs those exact seeds.
- **Immutable once it has runs.** A suite can be edited freely until its first run lands. After that, edits require cloning to a new suite (the clone can reference the parent for lineage).

### Data-model shape

Submission and Suite are independent. Their relationship is implicit through Runs:

```
Submission(id, player, artifact)
Suite(id, challenge_ref, parameters, seeds, scoring, ...)
Run(id, submission_id, suite_id, seed, score, ...)
```

A leaderboard for suite X = group X's runs by submission, apply suite's aggregation, rank. (An `aggregated_score` cache can be added later if needed; not part of v1.)

## Engine contract

We do **not** own the engine. Each challenge ships a third-party engine (binary or script) that we have no ability to modify. To absorb that variability, every challenge provides a **wrapper** alongside the engine, and the platform only ever talks to the wrapper.

**Per-run lifecycle (working-dir convention):**

1. Platform creates a fresh `runs/<id>/` directory.
2. Platform populates `runs/<id>/submission/` and `runs/<id>/inputs/`.
3. Platform invokes the wrapper: `./wrapper --workdir=runs/<id>`.
4. Wrapper calls the actual engine however it needs to and writes `runs/<id>/result.json`.
5. Platform reads `result.json`, records the score, and stores everything else in the run directory as opaque artifacts.

**Required result schema:** `{"score": <number>}`. Any additional fields are stored verbatim and ignored by the platform — challenge-layer code can interpret them.

**Challenge bundle layout:**

```
challenges/<name>/
├── engine/              # original engine, untouched
├── wrapper              # per-challenge adapter (script or binary)
└── manifest.{json,toml} # name, runtime config, defaults
```

**Bundled example: `challenges/regex-count/`**

A toy challenge included to anchor the contract. The seed is a 64-bit value in hex (`inputs/seed.txt`); the engine SHA-512s those 8 bytes, base64-encodes the digest, and counts matches of the player's regex (`submission/pattern.txt`) in the resulting ~88-char string. The wrapper bridges the platform's working-dir contract to the engine's CLI and writes `result.json`. Score = match count.

```sh
cd challenges/regex-count
make test    # builds engine, runs example workdir, writes result.json
```

## Auth

Single shared password gates the entire app — set via env var. No user accounts, no handle system, no roles. Once authenticated, the session has full access including admin actions (creating suites, triggering re-evaluations).

Submissions carry an optional free-text `submitter` field so the leaderboard can attribute scores; left blank, the entry shows as anonymous.

This is intentionally minimal for solo / small-group use. An optional `ADMIN_PASSWORD` to gate admin-only endpoints can be layered on later without changing the data model.

## Process model

The same binary runs in three modes:

```
ctf-evals serve                            # HTTP API + UI
ctf-evals worker                           # claims runs from the API and executes them
ctf-evals register-challenge <bundle-dir>  # operator-side: package + upload a challenge
```

**Only `serve` talks to the database.** Workers communicate exclusively with `serve` over HTTP using a worker token (`CTF_EVALS_WORKER_TOKEN`). This keeps the trust boundary tight: a compromised wrapper can only do what the internal API allows (claim/fetch/report), not arbitrary DB writes. It also means workers don't need DB credentials or network reachability to the DB — only to the API server.

**Bundle distribution.** Each challenge is packaged into a deterministic tarball, content-hashed (SHA-256), and stored as a blob on the `Challenge` row. Suites reference a specific challenge **version** (hash), so re-registering a challenge produces a new version row and existing suites keep referencing the old one — preserving suite immutability across upgrades. Workers fetch bundles lazily on first encounter and cache them locally at `$CTF_EVALS_CACHE_DIR/challenges/<version>/`.

**Run lifecycle:** `pending → claimed → running → succeeded | failed | timed_out`.

**Worker visibility + heartbeat.** Every call a worker makes to `/internal/*` upserts its row in the `workers` table (keyed by self-reported `X-Worker-Id`, defaulting to hostname). The row carries `last_seen` and `runs_handled`. The operator can see who's connected with `SELECT * FROM workers`. No explicit registration step — workers just need the shared `CTF_EVALS_WORKER_TOKEN`; the row appears on first call.

**Orphan-claim sweeper.** A goroutine in `serve` periodically reclaims `claimed` runs whose worker hasn't heart-beat in `CTF_EVALS_WORKER_TIMEOUT` (default 30s) — sets them back to `pending` so a live worker can pick them up. Defaults: sweep every 10s, treat workers stale after 30s of silence. Idle workers stay fresh because their poll loop calls `claim` every `PollInterval`.

## Scaling

Three independent ways to add throughput. Pick whichever matches your constraint.

### 1. Vertical — bigger per-worker concurrency

Each worker process runs a bounded goroutine pool sized by `CTF_EVALS_CONCURRENCY` (default 2). Each slot holds one in-flight run.

```sh
CTF_EVALS_CONCURRENCY=8 ./ctf-evals worker
```

Limit is the host's CPU and how much your subprocesses fight for resources. Cheapest knob.

### 2. Horizontal — more worker processes, same host

Workers are stateless beyond a local content-addressed bundle cache, so any number can share work safely. Each call to `/internal/runs/claim` is atomic (single-row `UPDATE` inside a tx; the server holds a mutex so SQLite doesn't `SQLITE_BUSY` itself).

With Docker Compose (the `worker` service in the bundled compose file is designed for this):

```sh
docker compose up --scale worker=4 -d
```

Bare metal — give each process a distinct ID so the `workers` table can tell them apart:

```sh
CTF_EVALS_WORKER_TOKEN=$TOKEN CTF_EVALS_WORKER_ID=alpha ./ctf-evals worker &
CTF_EVALS_WORKER_TOKEN=$TOKEN CTF_EVALS_WORKER_ID=beta  ./ctf-evals worker &
```

### 3. Horizontal — workers on other hosts

The API-mediated design (workers never touch the DB) makes this trivial. Each worker only needs to reach `serve`'s URL:

```sh
CTF_EVALS_SERVER_URL=https://serve.example.com:8080 \
CTF_EVALS_WORKER_TOKEN=$TOKEN \
./ctf-evals worker
```

Bundle distribution is content-hash-addressed: a fresh worker fetches each challenge bundle from `GET /internal/bundles/<sha256>` once, then caches at `$CTF_EVALS_CACHE_DIR/challenges/<version>/`. Submission artifacts are similarly fetched per-run from `/internal/submissions/<id>/artifact`.

### What scales without further work

- **Worker self-registration** is lazy — the `workers` table row appears on first call, throttled to one DB write per worker per 5 seconds. No admin step.
- **Orphan recovery** — a sweeper in `serve` reclaims `claimed` runs from workers whose `last_seen` is older than `CTF_EVALS_WORKER_TIMEOUT` (default 30s). Scale workers down (or kill them) and their in-flight claims return to the queue within `WORKER_TIMEOUT + SWEEP_INTERVAL`.
- **Live ETA** — `GET /api/queue` reports pending count, recent throughput (run-seconds completed per wall-second), and an ETA weighted by each suite's mean duration. Drops in real time as you add workers.

### Bottlenecks to expect

- **SQLite serialization.** All writes on `/internal/*` (claim, result, heartbeat, sweeper) are serialized through a server-side `sync.Mutex` to dodge `SQLITE_BUSY`. Past ~30 workers you'll see throughput plateau no matter how many workers you add. Cure: switch to Postgres with `CTF_EVALS_DB=postgres://…` — concurrent writes are native, and the mutex becomes uncontended overhead.
- **Server CPU on bundle fetches** when cold-starting a fleet against a freshly-registered challenge. Bundles are MB-scale and served straight from the DB row. If this ever matters in practice, the endpoint is pure-read on content-addressed data — fronting it with a CDN or read replica is safe.
- **Heartbeat write pressure scales with worker count.** Throttle is 5s per worker, so 100 workers ≈ 20 writes/sec to the `workers` table — fine for SQLite, trivial for Postgres. Bump `heartbeatTTL` in `server/heartbeat.go` if you go much bigger.

### Tuning knobs (worker side)

| Env var                     | Default | What it does                                                 |
| --------------------------- | ------- | ------------------------------------------------------------ |
| `CTF_EVALS_CONCURRENCY`     | `2`     | Parallel run slots per worker process.                        |
| `CTF_EVALS_POLL_INTERVAL`   | `1s`    | How often each worker calls `/internal/runs/claim`.           |
| `CTF_EVALS_WORKER_ID`       | hostname| Self-reported name in the `workers` table.                    |
| `CTF_EVALS_CACHE_DIR`       | `./data/cache` | Where extracted bundles and per-run workdirs live.    |
| `CTF_EVALS_SERVER_URL`      | `http://localhost:8080` | Where to find `serve`.                          |
| `CTF_EVALS_WORKER_TOKEN`    | (required) | Bearer token for `/internal/*`.                          |

### Tuning knobs (server side)

| Env var                     | Default | What it does                                                 |
| --------------------------- | ------- | ------------------------------------------------------------ |
| `CTF_EVALS_SWEEP_INTERVAL`  | `10s`   | How often the orphan-claim sweeper runs.                      |
| `CTF_EVALS_WORKER_TIMEOUT`  | `30s`   | Worker is considered dead after this much silence — their claims get returned to the queue. |
| `CTF_EVALS_MAX_UPLOAD_MB`   | `16`    | Per-submission artifact size cap.                             |
| `CTF_EVALS_DB`              | `sqlite://./data/ctf-evals.db?...` | Set to `postgres://…` to lift the SQLite serialization bottleneck. |

## Tradeoffs to be aware of

- Subprocess-only isolation (vs. per-run containers) means a runaway engine shares resources with its worker container and other concurrent runs. Mitigations: bounded per-worker concurrency, per-run timeouts, OS rlimits on CPU/memory/file size.
- SQLite is fine for single-worker local dev and small multi-worker setups (tested up to ~30 concurrent). Past that, the server-side write serialization becomes the bottleneck and Postgres is the right answer.

## Repository layout

```
cmd/ctf-evals/        # main entry — `serve` and `worker` subcommands
internal/
  config/             # env-driven config (DSN, addr, password, etc.)
  db/                 # opens Ent client, runs schema migration
  server/             # chi router, basic-auth middleware, SPA serving
  worker/             # poll loop, claim, subprocess executor
ent/                  # Ent schemas + generated client
  schema/             # Challenge, Suite, Submission, Run
challenges/           # per-challenge bundles (engine/ + wrapper + manifest)
  regex-count/        # bundled toy challenge
web/                  # React + Vite SPA
```

## Running locally

### Docker Compose (recommended)

```sh
cp .env.example .env             # then edit and set CTF_EVALS_PASSWORD + CTF_EVALS_WORKER_TOKEN
docker compose up --build        # one serve + one worker; toy challenge auto-registered
```

`http://localhost:8080/healthz` is public. Hit the UI at `http://localhost:8080/`, log in with whatever you set as `CTF_EVALS_PASSWORD` (any username).

Scale workers: `docker compose up --scale worker=3`.

Seed a quick demo run inside the running stack:

```sh
docker compose exec serve /app/ctf-evals seed-demo regex-count "[A-Z]" 5
```

### From source

```sh
go build ./cmd/ctf-evals
make -C challenges/regex-count           # builds the toy engine
mkdir -p data
./ctf-evals register-challenge ./challenges/regex-count

CTF_EVALS_PASSWORD=changeme CTF_EVALS_WORKER_TOKEN=devtoken ./ctf-evals serve     # terminal 1
CTF_EVALS_WORKER_TOKEN=devtoken ./ctf-evals worker                                 # terminal 2
cd web && npm install && npm run dev                                                # terminal 3 (SPA dev server)
```

Default DB is SQLite at `./data/ctf-evals.db` with WAL + busy_timeout. Set `CTF_EVALS_DB=postgres://...` for Postgres.

## HTTP API

All `/api/*` endpoints require HTTP Basic auth with the shared password (username is ignored).

| Method | Path                                | Purpose                                                |
| ------ | ----------------------------------- | ------------------------------------------------------ |
| GET    | `/healthz`                          | Liveness probe, public.                                |
| GET    | `/api/me`                           | Auth check.                                            |
| GET    | `/api/challenges`                   | List registered challenges (name, version, size).       |
| GET    | `/api/suites`                       | List all suites (most recent first).                   |
| POST   | `/api/suites`                       | Create a suite (challenge_id, seeds, optional scoring).|
| GET    | `/api/suites/{id}`                  | Suite detail.                                          |
| GET    | `/api/suites/{id}/leaderboard`      | Aggregated scores per submission, ranked.              |
| GET    | `/api/submissions`                  | List submissions.                                      |
| POST   | `/api/submissions`                  | Upload an artifact (`multipart/form-data`: one or more `suite_ids` fields, `submitter`, `artifact`). Server fans out one pending Run per (suite × seed) and seals each touched suite. |
| GET    | `/api/submissions/{id}`             | Submission detail with its runs.                       |
| GET    | `/api/runs/{id}`                    | Run detail incl. opaque `result` blob.                 |
| GET    | `/api/queue`                        | Global queue status: pending count, per-suite mean durations, recent throughput (run-seconds completed per wall-second), and ETA. Drives the live banner on the home page. |

Each leaderboard entry carries all five aggregates (`mean`, `median`, `mode`, `max`, `min`) plus `stddev`, a 95% CI half-width (`ci95_half_width = 1.96 × stddev / √n`), and `p_value_vs_leader` — a two-tailed paired t-test (normal approximation) against the top-mean submission, exploiting the fact that suites have fixed seeds. The SPA defaults to ranking by mean but every metric column is clickable to re-sort.

## Status

Usable for small-to-medium evals end-to-end:

- Full submission flow (register challenge → create suite → upload submission → ranked leaderboard) works from the SPA, no curl needed.
- Each leaderboard exposes per-submission distribution stats (mean, median, mode, max, min, stddev), 95% CIs, and paired t-test p-values against the leader.
- SubmissionDetail breaks out per-suite stats, percentiles (p10/p50/p90/p99), and a score-distribution histogram.
- Live queue ETA, per-suite mean run duration, worker self-registration, orphan-claim sweeper.
- One bundled toy challenge (`challenges/regex-count`) with a configurable `sleep` parameter for exercising the worker pool.

Known shortcuts / future work:
- Failure-only submissions get bucketed under "in flight" on the leaderboard (visually muted but logically dead). Error text isn't surfaced on the run table.
- No admin re-evaluation endpoint yet — you can manually `INSERT` Runs against a new (submission, suite) pair, or build a small endpoint.
- Wrapper stdout/stderr is currently discarded by the worker; a real deployment probably wants to persist these to `logs/` in the run's working directory and stream them via an admin endpoint.
- `SELECT … FOR UPDATE SKIP LOCKED` not used yet on Postgres — claims go through the same server-side mutex as SQLite. Fine up to moderate scale; rewriting `handleClaim` to use raw SKIP LOCKED would let workers claim concurrently on Postgres.

