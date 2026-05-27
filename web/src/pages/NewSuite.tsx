import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { api } from "../api/client";
import type { Challenge } from "../api/types";

export function NewSuite() {
  const navigate = useNavigate();

  const [challenges, setChallenges] = useState<Challenge[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [challengeId, setChallengeId] = useState<number | null>(null);
  const [seedsText, setSeedsText] = useState("");
  const [timeoutSecs, setTimeoutSecs] = useState(60);
  const [parametersText, setParametersText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .listChallenges()
      .then((rows) => {
        if (cancelled) return;
        setChallenges(rows);
        if (rows.length > 0) setChallengeId(rows[0].id);
      })
      .catch((err) => !cancelled && setError(err.message ?? String(err)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, []);

  const generateSeeds = (n: number) => {
    const buf = new Uint8Array(8);
    const lines: string[] = [];
    for (let i = 0; i < n; i++) {
      crypto.getRandomValues(buf);
      lines.push(
        Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join(""),
      );
    }
    setSeedsText(lines.join("\n"));
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (challengeId == null) {
      setError("Pick a challenge.");
      return;
    }
    const seeds = seedsText
      .split(/\r?\n/)
      .map((s) => s.trim())
      .filter(Boolean);
    if (seeds.length === 0) {
      setError("At least one seed is required.");
      return;
    }

    let parameters: Record<string, unknown> | undefined;
    const trimmedParams = parametersText.trim();
    if (trimmedParams) {
      try {
        parameters = JSON.parse(trimmedParams);
      } catch (err) {
        setError(
          "Parameters must be valid JSON: " +
            (err instanceof Error ? err.message : String(err)),
        );
        return;
      }
    }

    setSubmitting(true);
    try {
      const created = await api.createSuite({
        name: name.trim(),
        description: description.trim() || undefined,
        challenge_id: challengeId,
        seeds,
        timeout_seconds: timeoutSecs,
        parameters,
      });
      navigate(`/suites/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <span className="current">new suite</span>
      </nav>
      <main>
          <h1>New suite</h1>
          {error && <Alert title="Couldn't create suite">{error}</Alert>}
          {loading ? (
            <p className="t-cmt">loading challenges…</p>
          ) : challenges.length === 0 ? (
            <Alert title="No challenges registered">
              Run{" "}
              <code className="t-kw">ctf-evals register-challenge &lt;dir&gt;</code>{" "}
              on the host to register a bundle before creating a suite.
            </Alert>
          ) : (
            <form onSubmit={onSubmit} style={{ display: "grid", gap: "var(--space-4)", maxWidth: 640 }}>
              <Field label="Name" hint="Shown on the leaderboard.">
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </Field>

              <Field label="Description" hint="Optional.">
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </Field>

              <Field label="Challenge">
                <select
                  value={challengeId ?? ""}
                  onChange={(e) => setChallengeId(Number(e.target.value))}
                >
                  {challenges.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name} · {c.version.slice(0, 12)}…
                    </option>
                  ))}
                </select>
              </Field>

              <Field
                label="Seeds"
                hint="One per line. Each submission runs once per seed; fixed seeds make submissions directly comparable."
              >
                <textarea
                  rows={6}
                  value={seedsText}
                  onChange={(e) => setSeedsText(e.target.value)}
                  placeholder="853ca5f4873a1842&#10;a17b3c9e0f1d4528&#10;…"
                  required
                  style={{ width: "100%", fontFamily: "var(--mono)", fontSize: "13px" }}
                />
                <div style={{ display: "flex", gap: "var(--space-2)", marginTop: "var(--space-2)", flexWrap: "wrap" }}>
                  {[5, 10, 25, 100].map((n) => (
                    <button
                      key={n}
                      type="button"
                      className="btn secondary"
                      onClick={() => generateSeeds(n)}
                    >
                      Generate {n}
                    </button>
                  ))}
                </div>
              </Field>

              <Field label="Timeout per run" hint="Seconds.">
                <input
                  type="number"
                  min={1}
                  value={timeoutSecs}
                  onChange={(e) => setTimeoutSecs(Number(e.target.value))}
                />
              </Field>

              <Field
                label="Parameters"
                hint='Optional JSON object passed to the wrapper as CTF_PARAM_* env vars. e.g. {"sleep": 0.15}'
              >
                <textarea
                  rows={3}
                  value={parametersText}
                  onChange={(e) => setParametersText(e.target.value)}
                  placeholder='{"sleep": 0.15}'
                  style={{
                    width: "100%",
                    fontFamily: "var(--mono)",
                    fontSize: "13px",
                  }}
                />
              </Field>

              <div>
                <button type="submit" className="btn" disabled={submitting}>
                  {submitting ? "Creating…" : "Create suite"}
                </button>
              </div>
            </form>
          )}
      </main>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
      <label className="field-label">
        {label}
        {hint && <span className="hint">{hint}</span>}
      </label>
      {children}
    </div>
  );
}
