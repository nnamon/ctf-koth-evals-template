import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { SuiteBreakdown } from "../components/SuiteBreakdown";
import { api } from "../api/client";
import { subscribeEvents } from "../api/events";
import type { RunSummary, Suite, SubmissionDetail as Detail } from "../api/types";

export function SubmissionDetail() {
  const { id } = useParams();
  const subId = Number(id);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setDetail(await api.getSubmission(subId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [subId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Refetch on any server-sent run state change.
  useEffect(() => subscribeEvents(refresh), [refresh]);

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        {detail && detail.runs.length > 0 && (
          <>
            <Link to={`/suites/${detail.runs[0].suite_id}`}>
              suite-{detail.runs[0].suite_id}
            </Link>
            <span className="sep">/</span>
          </>
        )}
        <span className="current">submission {subId}</span>
      </nav>
      <main>
          {error && <Alert title="Error">{error}</Alert>}
          {!detail ? (
            <p className="t-cmt">loading…</p>
          ) : (
            <>
              <h1>
                {detail.name || <code className="t-cmt">unnamed</code>}
              </h1>
              <p>
                {detail.submitter && (
                  <>
                    submitted by <code className="t-cmt">{detail.submitter}</code>{" "}
                    ·{" "}
                  </>
                )}
                artifact <code className="t-path">{detail.artifact_name}</code>{" "}
                · <code className="t-num">{detail.artifact_size}</code> bytes ·{" "}
                <code className="t-num">
                  {new Date(detail.created_at).toLocaleString()}
                </code>
              </p>

              <Reevaluate submissionId={subId} onDone={refresh} />

              <BySuite runs={detail.runs} />

              <h2 style={{ marginTop: "var(--space-7)" }}>
                Recent runs
                {detail.runs.length > 100 && (
                  <span
                    className="t-cmt"
                    style={{ fontFamily: "var(--mono)", fontSize: 12, fontWeight: 400, letterSpacing: 0, textTransform: "none", marginLeft: "var(--space-3)" }}
                  >
                    showing last 100 of {detail.runs.length}
                  </span>
                )}
              </h2>
              <RunsTable runs={detail.runs.slice(-100).reverse()} />
              </>
          )}
      </main>
    </div>
  );
}

function Reevaluate({
  submissionId,
  onDone,
}: {
  submissionId: number;
  onDone: () => void;
}) {
  const [suites, setSuites] = useState<Suite[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    api.listSuites().then(setSuites).catch(() => {});
  }, []);

  const toggle = (id: number) =>
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  const onSubmit = async () => {
    if (selected.size === 0) return;
    setBusy(true);
    setMsg(null);
    try {
      const r = await api.reevaluateSubmission(submissionId, Array.from(selected));
      setMsg(`Queued ${r.runs_created} runs.`);
      setSelected(new Set());
      onDone();
    } catch (err) {
      setMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  if (suites.length === 0) return null;

  return (
    <details style={{ marginTop: "var(--space-5)" }}>
      <summary style={{ cursor: "pointer", fontFamily: "var(--mono)", fontSize: 13 }}>
        re-evaluate against another suite
      </summary>
      <div style={{ marginTop: "var(--space-3)", display: "grid", gap: "var(--space-2)", maxWidth: 520 }}>
        {suites.map((s) => (
          <label key={s.id} style={{ display: "flex", gap: "var(--space-2)", alignItems: "center", fontSize: 13 }}>
            <input
              type="checkbox"
              checked={selected.has(s.id)}
              onChange={() => toggle(s.id)}
            />
            {s.name}{" "}
            <code className="t-cmt">
              {s.challenge.name} · {s.seeds.length} seeds
            </code>
          </label>
        ))}
        <div style={{ display: "flex", gap: "var(--space-3)", alignItems: "center", marginTop: "var(--space-2)" }}>
          <button
            type="button"
            className="btn"
            onClick={onSubmit}
            disabled={busy || selected.size === 0}
          >
            {busy ? "Queuing…" : "Re-evaluate"}
          </button>
          {msg && <code className="t-cmt">{msg}</code>}
        </div>
      </div>
    </details>
  );
}

function BySuite({ runs }: { runs: RunSummary[] }) {
  const groups = useMemo(() => {
    const m = new Map<number, RunSummary[]>();
    for (const r of runs) {
      const k = r.suite_id;
      if (!m.has(k)) m.set(k, []);
      m.get(k)!.push(r);
    }
    return [...m.entries()].sort(([a], [b]) => a - b);
  }, [runs]);

  if (groups.length === 0) {
    return <p className="t-cmt">no runs yet</p>;
  }

  return (
    <div>
      {groups.map(([suiteId, rs]) => (
        <SuiteBreakdown key={suiteId} suiteId={suiteId} runs={rs} />
      ))}
    </div>
  );
}

function RunsTable({ runs }: { runs: Detail["runs"] }) {
  // Live-tick "now" so duration of running rows keeps climbing between polls.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const anyRunning = runs.some((r) =>
      ["claimed", "running"].includes(r.status),
    );
    if (!anyRunning) return;
    const t = setInterval(() => setNow(Date.now()), 200);
    return () => clearInterval(t);
  }, [runs]);

  return (
    <div className="data-scroll">
      <table className="data" style={{ width: "100%" }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left" }}>#</th>
            <th style={{ textAlign: "left" }}>Seed</th>
            <th style={{ textAlign: "left" }}>Status</th>
            <th style={{ textAlign: "right" }}>Score</th>
            <th style={{ textAlign: "right" }}>Duration</th>
            <th style={{ textAlign: "left" }}>Finished</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((r) => (
            <tr key={r.id}>
              <td>
                <Link to={`/runs/${r.id}`}>
                  <code className="t-num">{r.id}</code>
                </Link>
              </td>
              <td>
                <code className="t-path">{r.seed}</code>
              </td>
              <td>{statusBadge(r.status)}</td>
              <td style={{ textAlign: "right" }}>
                {r.score === undefined ? (
                  <code className="t-cmt">—</code>
                ) : (
                  <code className="t-num">{r.score}</code>
                )}
              </td>
              <td style={{ textAlign: "right" }}>
                <code className="t-num">{durationCell(r, now)}</code>
              </td>
              <td>
                {r.finished_at ? (
                  <code className="t-num">
                    {new Date(r.finished_at).toLocaleTimeString()}
                  </code>
                ) : (
                  <code className="t-cmt">—</code>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function durationCell(r: Detail["runs"][number], now: number): string {
  // Completed: finished_at - started_at.
  if (r.started_at && r.finished_at) {
    return formatDuration(
      new Date(r.finished_at).getTime() - new Date(r.started_at).getTime(),
    );
  }
  // Live: ticking from started_at (worker recorded it as it spawned).
  if (r.started_at) {
    return formatDuration(now - new Date(r.started_at).getTime());
  }
  return "—";
}

function formatDuration(ms: number): string {
  if (ms < 0) ms = 0;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`;
  const mins = Math.floor(ms / 60_000);
  const secs = Math.round((ms % 60_000) / 1000);
  return `${mins}m ${secs}s`;
}

function statusBadge(status: string) {
  const cls = {
    succeeded: "t-str",
    failed: "t-type",
    timed_out: "t-type",
    pending: "t-cmt",
    claimed: "t-kw",
    running: "t-kw",
  }[status] ?? "t-cmt";
  return <code className={cls}>{status}</code>;
}
