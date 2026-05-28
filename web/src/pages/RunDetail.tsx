import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { api } from "../api/client";
import type { RunDetail as Detail } from "../api/types";

export function RunDetail() {
  const { id } = useParams();
  const runId = Number(id);
  const [run, setRun] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setRun(await api.getRun(runId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [runId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Poll while unresolved.
  useEffect(() => {
    if (!run) return;
    if (!["pending", "claimed", "running"].includes(run.status)) return;
    const t = setInterval(refresh, 1000);
    return () => clearInterval(t);
  }, [run, refresh]);

  const duration =
    run?.started_at && run?.finished_at
      ? `${(
          (new Date(run.finished_at).getTime() -
            new Date(run.started_at).getTime()) /
          1000
        ).toFixed(2)}s`
      : "—";

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        {run && (
          <>
            <Link to={`/submissions/${run.submission_id}`}>
              {run.submission_name || `submission ${run.submission_id}`}
            </Link>
            <span className="sep">/</span>
          </>
        )}
        <span className="current">run {runId}</span>
      </nav>
      <main>
        {error && <Alert title="Error">{error}</Alert>}
        {!run ? (
          <p className="t-cmt">loading…</p>
        ) : (
          <>
            <h1>run #{run.id}</h1>
            <p>
              status {statusBadge(run.status)} ·{" "}
              {run.suite_id ? (
                <>
                  suite <Link to={`/suites/${run.suite_id}`}>#{run.suite_id}</Link> ·{" "}
                </>
              ) : null}
              seed <code className="t-path">{run.seed}</code> · score{" "}
              {run.score === undefined ? (
                <code className="t-cmt">—</code>
              ) : (
                <code className="t-num">{run.score}</code>
              )}{" "}
              · duration <code className="t-num">{duration}</code>
              {run.worker_id && (
                <>
                  {" "}
                  · worker <code className="t-cmt">{run.worker_id}</code>
                </>
              )}
            </p>

            {run.error && (
              <Alert title="Run error" variant="error">
                <code className="t-type">{run.error}</code>
              </Alert>
            )}

            <h2>stderr</h2>
            <LogBlock text={run.stderr} />

            <h2>stdout</h2>
            <LogBlock text={run.stdout} />

            {run.result && Object.keys(run.result).length > 0 && (
              <>
                <h2>result.json</h2>
                <pre>
                  <code>{JSON.stringify(run.result, null, 2)}</code>
                </pre>
              </>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function LogBlock({ text }: { text?: string }) {
  if (!text) {
    return <p className="t-cmt">(empty)</p>;
  }
  return (
    <pre style={{ maxHeight: 360, overflow: "auto" }}>
      <code>{text}</code>
    </pre>
  );
}

function statusBadge(status: string) {
  const cls =
    {
      succeeded: "t-str",
      failed: "t-type",
      timed_out: "t-type",
      cancelled: "t-cmt",
      pending: "t-cmt",
      claimed: "t-kw",
      running: "t-kw",
    }[status] ?? "t-cmt";
  return <code className={cls}>{status}</code>;
}
