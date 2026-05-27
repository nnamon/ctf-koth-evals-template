import { Link } from "react-router-dom";
import { Histogram } from "./Histogram";
import { max, mean, median, min, percentile, stddev } from "../lib/stats";
import type { RunSummary } from "../api/types";

type Props = {
  suiteId: number;
  runs: RunSummary[];
};

// SuiteBreakdown summarises every run a submission produced against one
// suite: counts, score percentiles, duration mean, score histogram.
export function SuiteBreakdown({ suiteId, runs }: Props) {
  const scores = runs
    .filter((r) => r.status === "succeeded" && r.score !== undefined)
    .map((r) => r.score as number);

  const durations = runs
    .filter((r) => r.started_at && r.finished_at)
    .map(
      (r) =>
        new Date(r.finished_at!).getTime() - new Date(r.started_at!).getTime(),
    );

  const succeeded = runs.filter((r) => r.status === "succeeded").length;
  const failed = runs.filter(
    (r) => r.status === "failed" || r.status === "timed_out",
  ).length;
  const inFlight = runs.filter((r) =>
    ["pending", "claimed", "running"].includes(r.status),
  ).length;
  const total = runs.length;

  const stats = {
    n: scores.length,
    mean: mean(scores),
    median: median(scores),
    stddev: stddev(scores),
    min: min(scores),
    max: max(scores),
    p10: percentile(scores, 10),
    p50: percentile(scores, 50),
    p90: percentile(scores, 90),
    p99: percentile(scores, 99),
    meanDuration: mean(durations),
  };

  return (
    <section style={{ marginTop: "var(--space-6)" }}>
      <h3
        style={{
          fontFamily: "var(--sans)",
          fontWeight: 700,
          fontSize: 14,
          textTransform: "uppercase",
          letterSpacing: "0.14em",
          borderBottom: "1px solid var(--rule)",
          paddingBottom: "var(--space-2)",
          marginBottom: "var(--space-3)",
          display: "flex",
          gap: "var(--space-3)",
          alignItems: "baseline",
        }}
      >
        <Link to={`/suites/${suiteId}`} style={{ color: "var(--accent)" }}>
          suite-{suiteId}
        </Link>
        <span style={{ fontFamily: "var(--mono)", fontWeight: 400, fontSize: 12, letterSpacing: 0, textTransform: "none", color: "var(--text-muted)" }}>
          {succeeded}/{total} succeeded
          {failed > 0 && (
            <>
              {" · "}
              <code className="t-type">{failed} failed</code>
            </>
          )}
          {inFlight > 0 && (
            <>
              {" · "}
              <code className="t-kw">{inFlight} in flight</code>
            </>
          )}
        </span>
      </h3>

      <div className="kpis" style={{ marginBottom: "var(--space-4)" }}>
        <Stat label="mean" value={stats.mean} />
        <Stat label="median" value={stats.median} />
        <Stat label="stddev" value={stats.stddev} />
        <Stat
          label="mean duration"
          value={stats.meanDuration}
          format={formatMs}
        />
      </div>

      <div className="kpis" style={{ marginBottom: "var(--space-4)" }}>
        <Stat label="p10" value={stats.p10} />
        <Stat label="p50" value={stats.p50} />
        <Stat label="p90" value={stats.p90} />
        <Stat label="p99" value={stats.p99} />
      </div>

      {stats.n >= 5 && (
        <Histogram
          values={scores}
          caption={`score distribution · ${stats.n} runs · mean (purple dashed) ${formatScore(
            stats.mean,
          )} · median (cyan dashed) ${formatScore(stats.median)}`}
        />
      )}
    </section>
  );
}

function Stat({
  label,
  value,
  format,
}: {
  label: string;
  value: number;
  format?: (n: number) => string;
}) {
  return (
    <div className="kpi">
      <div className="kpi-label">{label}</div>
      <div className="kpi-value">
        {Number.isFinite(value) ? (
          <code className="t-num">{(format ?? formatScore)(value)}</code>
        ) : (
          <code className="t-cmt">—</code>
        )}
      </div>
    </div>
  );
}

function formatScore(n: number): string {
  if (Number.isInteger(n)) return String(n);
  return n.toFixed(2);
}

function formatMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
  return `${Math.round(ms / 1000)}s`;
}
