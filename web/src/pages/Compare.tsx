import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { OverlayHistogram, type Series } from "../components/OverlayHistogram";
import { api } from "../api/client";
import { subscribeEvents } from "../api/events";
import { max, mean, median, min, percentile, stddev } from "../lib/stats";
import type { SubmissionDistribution, Suite } from "../api/types";

// Distinguishable on both themes; assigned by stable submission order.
const PALETTE = [
  "#d6409f",
  "#0aa5b3",
  "#e5a000",
  "#7c6cff",
  "#3fa34d",
  "#e5484d",
  "#0090ff",
  "#bd5b00",
];

export function Compare() {
  const { id } = useParams();
  const suiteId = Number(id);

  const [suite, setSuite] = useState<Suite | null>(null);
  const [dists, setDists] = useState<SubmissionDistribution[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [seeded, setSeeded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [s, d] = await Promise.all([
        api.getSuite(suiteId),
        api.distributions(suiteId),
      ]);
      setSuite(s);
      setDists(d);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [suiteId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => subscribeEvents(refresh), [refresh]);

  // Default selection: top 4 by mean (distributions arrive pre-sorted). Seed
  // once so live refetches don't clobber the user's choices.
  useEffect(() => {
    if (seeded || dists.length === 0) return;
    setSelected(new Set(dists.slice(0, 4).map((d) => d.submission_id)));
    setSeeded(true);
  }, [dists, seeded]);

  const colorOf = useMemo(() => {
    const m = new Map<number, string>();
    dists.forEach((d, i) => m.set(d.submission_id, PALETTE[i % PALETTE.length]));
    return m;
  }, [dists]);

  const toggle = (sid: number) =>
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(sid) ? next.delete(sid) : next.add(sid);
      return next;
    });

  const series: Series[] = dists
    .filter((d) => selected.has(d.submission_id))
    .map((d) => ({
      id: d.submission_id,
      label: d.name || `#${d.submission_id}`,
      color: colorOf.get(d.submission_id) ?? "var(--accent)",
      values: d.scores,
    }));

  return (
    <div className="page wide">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <Link to={`/suites/${suiteId}`}>{suite ? suite.name : `suite-${suiteId}`}</Link>
        <span className="sep">/</span>
        <span className="current">compare</span>
      </nav>
      <main>
        <h1>Compare submissions</h1>
        {suite && (
          <p>
            Challenge <code className="t-type">{suite.challenge.name}</code> ·{" "}
            <code className="t-num">{suite.seeds.length}</code> seeds · overlaying{" "}
            <code className="t-num">{series.length}</code> of{" "}
            <code className="t-num">{dists.length}</code> submissions
          </p>
        )}

        {error && <Alert title="Error">{error}</Alert>}

        {dists.length === 0 ? (
          <p className="t-cmt">
            no completed runs yet —{" "}
            <Link to={`/suites/${suiteId}`}>back to suite</Link>
          </p>
        ) : (
          <>
            <div className="compare-picker">
              {dists.map((d) => {
                const color = colorOf.get(d.submission_id)!;
                const on = selected.has(d.submission_id);
                return (
                  <label
                    key={d.submission_id}
                    className={on ? "compare-chip on" : "compare-chip"}
                  >
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={() => toggle(d.submission_id)}
                    />
                    <span
                      className="swatch"
                      style={{ background: on ? color : "transparent", borderColor: color }}
                    />
                    <span className="compare-chip-name">
                      {d.name || `#${d.submission_id}`}
                    </span>
                    <code className="t-cmt">
                      μ{formatScore(mean(d.scores))} · n{d.scores.length}
                    </code>
                  </label>
                );
              })}
            </div>

            {series.length === 0 ? (
              <p className="t-cmt">select one or more submissions to overlay</p>
            ) : (
              <>
                <OverlayHistogram series={series} />
                <ComparisonTable dists={dists.filter((d) => selected.has(d.submission_id))} colorOf={colorOf} />
              </>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function ComparisonTable({
  dists,
  colorOf,
}: {
  dists: SubmissionDistribution[];
  colorOf: Map<number, string>;
}) {
  return (
    <div className="data-scroll" style={{ marginTop: "var(--space-6)" }}>
      <table className="data" style={{ width: "100%" }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left" }}>Submission</th>
            <th style={{ textAlign: "right" }}>n</th>
            <th style={{ textAlign: "right" }}>mean</th>
            <th style={{ textAlign: "right" }}>median</th>
            <th style={{ textAlign: "right" }}>stddev</th>
            <th style={{ textAlign: "right" }}>min</th>
            <th style={{ textAlign: "right" }}>p10</th>
            <th style={{ textAlign: "right" }}>p90</th>
            <th style={{ textAlign: "right" }}>max</th>
          </tr>
        </thead>
        <tbody>
          {dists.map((d) => (
            <tr key={d.submission_id}>
              <td>
                <span
                  className="swatch"
                  style={{ background: colorOf.get(d.submission_id), borderColor: colorOf.get(d.submission_id) }}
                />{" "}
                <Link to={`/submissions/${d.submission_id}`}>
                  {d.name || <code className="t-cmt">#{d.submission_id}</code>}
                </Link>
              </td>
              <td style={{ textAlign: "right" }}>
                <code className="t-num">{d.scores.length}</code>
              </td>
              <Num v={mean(d.scores)} />
              <Num v={median(d.scores)} />
              <Num v={stddev(d.scores)} />
              <Num v={min(d.scores)} />
              <Num v={percentile(d.scores, 10)} />
              <Num v={percentile(d.scores, 90)} />
              <Num v={max(d.scores)} />
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Num({ v }: { v: number }) {
  return (
    <td style={{ textAlign: "right" }}>
      {Number.isFinite(v) ? (
        <code className="t-num">{formatScore(v)}</code>
      ) : (
        <code className="t-cmt">—</code>
      )}
    </td>
  );
}

function formatScore(n: number): string {
  if (!Number.isFinite(n)) return "—";
  if (Number.isInteger(n)) return String(n);
  return n.toFixed(2);
}
