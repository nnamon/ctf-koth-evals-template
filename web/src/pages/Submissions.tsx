import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { api } from "../api/client";
import type { RunCounts, SubmissionSummary } from "../api/types";

export function Submissions() {
  const [items, setItems] = useState<SubmissionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<number | null>(null);

  const refresh = useCallback(async () => {
    try {
      setItems(await api.listSubmissions());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Poll while anything has unresolved runs so the status pills stay live.
  useEffect(() => {
    const anyInFlight = items.some((s) => s.runs.pending > 0);
    if (!anyInFlight) return;
    const t = setInterval(refresh, 1500);
    return () => clearInterval(t);
  }, [items, refresh]);

  const run =
    (op: (id: number) => Promise<unknown>) =>
    async (id: number) => {
      setBusy(id);
      try {
        await op(id);
        await refresh();
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setBusy(null);
      }
    };

  return (
    <div className="page wide">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <span className="current">submissions</span>
      </nav>
      <main>
        <h1>Submissions</h1>
        {error && <Alert title="Error">{error}</Alert>}
        {loading ? (
          <p className="t-cmt">loading…</p>
        ) : items.length === 0 ? (
          <p className="t-cmt">none yet</p>
        ) : (
          <div className="data-scroll">
            <table className="data" style={{ width: "100%" }}>
              <thead>
                <tr>
                  <th style={{ textAlign: "left" }}>#</th>
                  <th style={{ textAlign: "left" }}>Name</th>
                  <th style={{ textAlign: "left" }}>Submitter</th>
                  <th style={{ textAlign: "left" }}>Runs</th>
                  <th style={{ textAlign: "left" }}>Submitted</th>
                  <th style={{ textAlign: "right" }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <code className="t-num">{s.id}</code>
                    </td>
                    <td>
                      <Link to={`/submissions/${s.id}`}>
                        {s.name || <code className="t-cmt">unnamed</code>}
                      </Link>
                    </td>
                    <td>
                      {s.submitter ? (
                        <code className="t-cmt">{s.submitter}</code>
                      ) : (
                        <code className="t-cmt">—</code>
                      )}
                    </td>
                    <td>
                      <RunPills counts={s.runs} />
                    </td>
                    <td>
                      <code className="t-num">
                        {new Date(s.created_at).toLocaleString()}
                      </code>
                    </td>
                    <td style={{ textAlign: "right" }}>
                      <Actions
                        counts={s.runs}
                        busy={busy === s.id}
                        onPrioritize={run(api.prioritizeSubmission).bind(null, s.id)}
                        onCancel={run(api.cancelSubmission).bind(null, s.id)}
                        onRetry={run(api.retrySubmission).bind(null, s.id)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  );
}

function RunPills({ counts }: { counts: RunCounts }) {
  const { total, succeeded, pending, failed, timed_out, cancelled } = counts;
  if (total === 0) {
    return <code className="t-cmt">no runs</code>;
  }
  return (
    <span style={{ display: "inline-flex", gap: "var(--space-2)", flexWrap: "wrap" }}>
      <code className="t-num">{succeeded}/{total}</code>
      {pending > 0 && (
        <code className="t-kw">{pending} running</code>
      )}
      {failed > 0 && <code className="t-type">{failed} failed</code>}
      {timed_out > 0 && <code className="t-type">{timed_out} timed out</code>}
      {cancelled > 0 && <code className="t-cmt">{cancelled} cancelled</code>}
    </span>
  );
}

function Actions({
  counts,
  busy,
  onPrioritize,
  onCancel,
  onRetry,
}: {
  counts: RunCounts;
  busy: boolean;
  onPrioritize: () => void;
  onCancel: () => void;
  onRetry: () => void;
}) {
  const hasPending = counts.pending > 0;
  const hasRetryable = counts.failed + counts.timed_out + counts.cancelled > 0;
  return (
    <span style={{ display: "inline-flex", gap: "var(--space-2)", justifyContent: "flex-end" }}>
      <button
        type="button"
        className="btn secondary action-btn"
        title="Send this submission's pending runs to the top of the queue"
        disabled={!hasPending || busy}
        onClick={onPrioritize}
      >
        ↑
      </button>
      <button
        type="button"
        className="btn secondary action-btn"
        title="Cancel pending / claimed runs (running ones complete on their own)"
        disabled={!hasPending || busy}
        onClick={onCancel}
      >
        ✕
      </button>
      <button
        type="button"
        className="btn secondary action-btn"
        title="Re-queue failed / timed-out / cancelled runs"
        disabled={!hasRetryable || busy}
        onClick={onRetry}
      >
        ↻
      </button>
    </span>
  );
}
