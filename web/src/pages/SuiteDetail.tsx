import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { Leaderboard } from "../components/Leaderboard";
import { api } from "../api/client";
import type { LeaderboardEntry, Suite } from "../api/types";

export function SuiteDetail() {
  const params = useParams();
  const id = Number(params.id);

  const [suite, setSuite] = useState<Suite | null>(null);
  const [board, setBoard] = useState<LeaderboardEntry[]>([]);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [s, b] = await Promise.all([api.getSuite(id), api.leaderboard(id)]);
      setSuite(s);
      setBoard(b);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [id]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    const inFlight = board.some(
      (e) => e.runs.pending > 0 || e.runs.total === 0,
    );
    if (!inFlight) return;
    const t = setInterval(refresh, 2000);
    return () => clearInterval(t);
  }, [board, refresh]);

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <span className="current">{suite ? suite.name : `suite-${id}`}</span>
      </nav>
      <main>
        {error && <Alert title="Error">{error}</Alert>}
        {!suite ? (
          <p className="t-cmt">loading…</p>
        ) : (
          <>
            <div className="page-header">
              <h1>{suite.name}</h1>
              <Link to={`/submit?suite=${id}`} className="btn">
                Submit
              </Link>
            </div>
            <p>
              Challenge <code className="t-type">{suite.challenge.name}</code>{" "}
              · version{" "}
              <code className="t-path" title={suite.challenge.version}>
                {suite.challenge.version.slice(0, 12)}…
              </code>{" "}
              · <code className="t-num">{suite.seeds.length}</code> seeds ·
              timeout <code className="t-num">{suite.timeout_seconds}s</code> ·{" "}
              {suite.sealed ? (
                <code className="t-str">sealed</code>
              ) : (
                <code className="t-kw">open</code>
              )}
            </p>

            <h2>Leaderboard</h2>
            {board.length === 0 ? (
              <p className="t-cmt">no submissions yet</p>
            ) : (
              <Leaderboard entries={board} />
            )}
          </>
        )}
      </main>
    </div>
  );
}
