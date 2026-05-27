import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { Leaderboard } from "../components/Leaderboard";
import { api } from "../api/client";
import type { LeaderboardEntry, QueueStatus, Suite } from "../api/types";

export function Overview() {
  const [params, setParams] = useSearchParams();
  const requested = Number(params.get("suite")) || null;

  const [suites, setSuites] = useState<Suite[]>([]);
  const [loadingSuites, setLoadingSuites] = useState(true);
  const [board, setBoard] = useState<LeaderboardEntry[]>([]);
  const [boardError, setBoardError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [queue, setQueue] = useState<QueueStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .listSuites()
      .then((rows) => !cancelled && setSuites(rows))
      .catch((err) => !cancelled && setError(err.message ?? String(err)))
      .finally(() => !cancelled && setLoadingSuites(false));
    return () => {
      cancelled = true;
    };
  }, []);

  const selected = useMemo<Suite | null>(() => {
    if (suites.length === 0) return null;
    if (requested && suites.some((s) => s.id === requested)) {
      return suites.find((s) => s.id === requested) ?? null;
    }
    return suites[0]; // most recent (server returns desc by created_at)
  }, [suites, requested]);

  const refreshBoard = useCallback(async () => {
    if (!selected) return;
    try {
      const [b, q] = await Promise.all([api.leaderboard(selected.id), api.queue()]);
      setBoard(b);
      setQueue(q);
    } catch (err) {
      setBoardError(err instanceof Error ? err.message : String(err));
    }
  }, [selected]);

  useEffect(() => {
    setBoard([]);
    setBoardError(null);
    refreshBoard();
  }, [refreshBoard]);

  // Live-update while any entry is still running. 1s feels right for the
  // human eye without hammering the DB on 500-seed suites.
  useEffect(() => {
    if (!selected) return;
    const inFlight = board.some(
      (e) => e.runs.pending > 0 || !e.metrics,
    );
    if (!inFlight) return;
    const t = setInterval(refreshBoard, 1000);
    return () => clearInterval(t);
  }, [selected, board, refreshBoard]);

  const pickSuite = (id: number) => {
    setParams({ suite: String(id) }, { replace: true });
  };

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <span className="current">leaderboard</span>
      </nav>
      <main>
        <div className="page-header">
          <h1>{selected ? selected.name : "Leaderboard"}</h1>
          <Link to="/suites/new" className="btn">
            New suite
          </Link>
        </div>

        {selected && (
          <p>
            Challenge{" "}
            <code className="t-type">{selected.challenge.name}</code> ·{" "}
            <code className="t-num">{selected.seeds.length}</code> seeds ·{" "}
            <Link to={`/suites/${selected.id}`}>suite details →</Link>
          </p>
        )}

        {error && <Alert title="Couldn't load suites">{error}</Alert>}

        {loadingSuites ? (
          <p className="t-cmt">loading…</p>
        ) : suites.length === 0 ? (
          <Alert title="No suites yet">
            <Link to="/suites/new">Create one</Link> to start accepting submissions.
          </Alert>
        ) : (
          <>
            {suites.length > 1 && (
              <div className="suite-chips">
                {suites.map((s) => (
                  <button
                    key={s.id}
                    type="button"
                    className={
                      selected && s.id === selected.id
                        ? "chip active"
                        : "chip"
                    }
                    onClick={() => pickSuite(s.id)}
                  >
                    {s.name}
                  </button>
                ))}
              </div>
            )}

            <QueueBanner status={queue} />

            {boardError && <Alert title="Couldn't load leaderboard">{boardError}</Alert>}

            {selected && board.length === 0 ? (
              <p className="t-cmt">no submissions yet — be the first via{" "}
                <Link to={`/submit?suite=${selected.id}`}>/submit</Link>.
              </p>
            ) : (
              <Leaderboard entries={board} />
            )}
          </>
        )}
      </main>
    </div>
  );
}

function QueueBanner({ status }: { status: QueueStatus | null }) {
  if (!status || status.pending_total === 0) return null;

  const eta = status.eta_seconds ?? null;
  return (
    <div className="queue-banner">
      <span>
        <code className="t-num">{status.pending_total}</code> runs queued ·{" "}
        <code className="t-num">{status.throughput_per_sec.toFixed(1)}</code>{" "}
        run-s/s ·{" "}
        <code className="t-num">{status.active_workers}</code> worker
        {status.active_workers === 1 ? "" : "s"}
      </span>
      <span>
        ETA{" "}
        {eta === null ? (
          <code className="t-cmt">computing…</code>
        ) : (
          <code className="t-kw">{formatEta(eta)}</code>
        )}
      </span>
    </div>
  );
}

function formatEta(seconds: number): string {
  if (seconds < 1) return "<1s";
  if (seconds < 60) return `~${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const m = Math.floor(seconds / 60);
    const s = Math.round(seconds % 60);
    return `~${m}m ${s}s`;
  }
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  return `~${h}h ${m}m`;
}

