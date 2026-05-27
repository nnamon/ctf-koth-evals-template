import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { LeaderboardEntry, Metrics } from "../api/types";

type MetricKey = keyof Metrics;
const METRIC_COLS: MetricKey[] = ["mean", "median", "mode", "max", "min", "stddev"];

const COL_COUNT = 2 + METRIC_COLS.length + 4; // # | name | ...metrics | ci95 | p | runs | submitted

function isInFlight(e: LeaderboardEntry): boolean {
  return e.runs.pending > 0 || !e.metrics;
}

export function Leaderboard({ entries }: { entries: LeaderboardEntry[] }) {
  const [sortKey, setSortKey] = useState<MetricKey>("mean");
  const [direction, setDirection] = useState<"desc" | "asc">("desc");

  const { ranked, inFlight } = useMemo(() => {
    const ranked = entries.filter((e) => !isInFlight(e));
    const inFlight = entries.filter(isInFlight);
    const sign = direction === "desc" ? -1 : 1;
    ranked.sort((a, b) => {
      const av = a.metrics![sortKey];
      const bv = b.metrics![sortKey];
      if (av !== bv) return (av - bv) * sign;
      return (
        new Date(a.submitted_at).getTime() -
        new Date(b.submitted_at).getTime()
      );
    });
    inFlight.sort(
      (a, b) =>
        new Date(a.submitted_at).getTime() -
        new Date(b.submitted_at).getTime(),
    );
    return { ranked, inFlight };
  }, [entries, sortKey, direction]);

  const onHeaderClick = (key: MetricKey) => {
    if (key === sortKey) {
      setDirection((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortKey(key);
      setDirection("desc");
    }
  };

  return (
    <div className="data-scroll">
      <table className="data leaderboard" style={{ width: "100%" }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left" }}>#</th>
            <th style={{ textAlign: "left" }}>Name</th>
            {METRIC_COLS.map((key) => (
              <th
                key={key}
                onClick={() => onHeaderClick(key)}
                className={key === sortKey ? "sort-active" : "sortable"}
                style={{ textAlign: "right", cursor: "pointer" }}
                title={`Sort by ${key}`}
              >
                {key}
                {key === sortKey && (
                  <span className="sort-arrow">
                    {" "}
                    {direction === "desc" ? "↓" : "↑"}
                  </span>
                )}
              </th>
            ))}
            <th style={{ textAlign: "right" }} title="95% CI half-width around the mean">
              ci95
            </th>
            <th
              style={{ textAlign: "right" }}
              title="Two-tailed paired t-test p-value vs the top-mean submission"
            >
              p vs lead
            </th>
            <th style={{ textAlign: "left" }}>Runs</th>
            <th style={{ textAlign: "left" }}>Submitted</th>
          </tr>
        </thead>
        <tbody>
          {ranked.map((e, i) => (
            <tr key={e.submission_id}>
              <td>
                <code className="t-num">{i + 1}</code>
              </td>
              <td>{nameCell(e)}</td>
              {METRIC_COLS.map((key) => (
                <td
                  key={key}
                  style={{ textAlign: "right" }}
                  className={key === sortKey ? "metric-active" : undefined}
                >
                  <code className="t-num">{formatScore(e.metrics![key])}</code>
                </td>
              ))}
              <td style={{ textAlign: "right" }}>
                {e.ci95_half_width > 0 ? (
                  <code className="t-num">
                    ±{formatScore(e.ci95_half_width)}
                  </code>
                ) : (
                  <code className="t-cmt">—</code>
                )}
              </td>
              <td style={{ textAlign: "right" }}>{pValueCell(e, i)}</td>
              <td>{runsCell(e)}</td>
              <td>
                <code className="t-num">{relativeTime(e.submitted_at)}</code>
              </td>
            </tr>
          ))}

          {inFlight.length > 0 && ranked.length > 0 && (
            <tr className="leaderboard-divider">
              <td colSpan={COL_COUNT}>
                <code className="t-cmt">in flight</code>
              </td>
            </tr>
          )}

          {inFlight.map((e) => (
            <tr key={e.submission_id} className="in-flight-row">
              <td>
                <code className="t-cmt">·</code>
              </td>
              <td>{nameCell(e)}</td>
              {METRIC_COLS.map((key) => (
                <td key={key} style={{ textAlign: "right" }}>
                  <code className="t-cmt">—</code>
                </td>
              ))}
              <td style={{ textAlign: "right" }}>
                <code className="t-cmt">—</code>
              </td>
              <td style={{ textAlign: "right" }}>
                <code className="t-cmt">—</code>
              </td>
              <td>{runsCell(e)}</td>
              <td>
                <code className="t-num">{relativeTime(e.submitted_at)}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function pValueCell(e: LeaderboardEntry, index: number): React.ReactNode {
  if (index === 0) {
    return <code className="t-str">leader</code>;
  }
  if (e.p_value_vs_leader === undefined) {
    return <code className="t-cmt">—</code>;
  }
  const p = e.p_value_vs_leader;
  // Bucket into useful, eyeballable categories. Significant gap from the
  // leader → red; ambiguous → muted; clearly tied → "tied".
  let label: string;
  let cls: string;
  if (p < 0.001) {
    label = "p<0.001";
    cls = "t-type";
  } else if (p < 0.01) {
    label = `p=${p.toFixed(3)}`;
    cls = "t-type";
  } else if (p < 0.05) {
    label = `p=${p.toFixed(3)}`;
    cls = "t-num";
  } else if (p > 0.5) {
    label = "tied";
    cls = "t-cmt";
  } else {
    label = `p=${p.toFixed(2)}`;
    cls = "t-cmt";
  }
  return <code className={cls}>{label}</code>;
}

function nameCell(e: LeaderboardEntry): React.ReactNode {
  return (
    <>
      <Link to={`/submissions/${e.submission_id}`}>
        {e.name || <code className="t-cmt">unnamed</code>}
      </Link>
      {e.submitter && (
        <>
          {" "}
          <code className="t-cmt">{e.submitter}</code>
        </>
      )}
    </>
  );
}

function runsCell(e: LeaderboardEntry): React.ReactNode {
  const { total, succeeded, failed, timed_out, pending } = e.runs;
  const count = (
    <code className="t-num">
      {succeeded}/{total}
    </code>
  );
  if (pending > 0) {
    return (
      <>
        {count} <code className="t-kw">running</code>
      </>
    );
  }
  if (failed > 0 || timed_out > 0) {
    const bits: string[] = [];
    if (failed > 0) bits.push(`${failed} failed`);
    if (timed_out > 0) bits.push(`${timed_out} timed out`);
    return (
      <>
        {count} <code className="t-type">{bits.join(", ")}</code>
      </>
    );
  }
  return (
    <>
      {count} <code className="t-str">complete</code>
    </>
  );
}

function formatScore(n: number): string {
  if (Number.isInteger(n)) return String(n);
  return n.toFixed(2);
}

function relativeTime(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  if (ms < 45_000) return "just now";
  if (ms < 3_600_000) return rtf.format(-Math.round(ms / 60_000), "minute");
  if (ms < 86_400_000) return rtf.format(-Math.round(ms / 3_600_000), "hour");
  return rtf.format(-Math.round(ms / 86_400_000), "day");
}
