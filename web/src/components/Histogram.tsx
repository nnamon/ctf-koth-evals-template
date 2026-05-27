import { useMemo } from "react";
import { histogram, mean, median } from "../lib/stats";

type Props = {
  values: number[];
  binCount?: number;
  height?: number;
  /** label shown beneath the chart */
  caption?: string;
  /** override bar color (defaults to --accent). */
  color?: string;
};

// Histogram renders an SVG bar chart of binned values, with thin mean/median
// markers and tick labels at min / max / bin edges. Kept minimal so the
// soft-brutalist-document aesthetic isn't disturbed.
export function Histogram({
  values,
  binCount = 14,
  height = 120,
  caption,
  color = "var(--accent)",
}: Props) {
  const view = useMemo(() => {
    const bins = histogram(values, binCount);
    if (bins.length === 0) return null;
    const maxCount = Math.max(...bins.map((b) => b.count));
    return { bins, maxCount, mean: mean(values), median: median(values) };
  }, [values, binCount]);

  if (!view) {
    return <p className="t-cmt">no data</p>;
  }

  const W = 800; // viewBox width; SVG scales to container
  const H = height;
  const padL = 28;
  const padR = 8;
  const padT = 6;
  const padB = 22;
  const innerW = W - padL - padR;
  const innerH = H - padT - padB;

  const xScale = (v: number) =>
    padL + ((v - view.bins[0].x0) / (view.bins.at(-1)!.x1 - view.bins[0].x0)) * innerW;
  const yScale = (count: number) =>
    padT + innerH - (count / view.maxCount) * innerH;

  return (
    <figure style={{ margin: 0 }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        width="100%"
        height={H}
        role="img"
        aria-label={caption ?? "histogram"}
      >
        {/* baseline */}
        <line
          x1={padL}
          x2={W - padR}
          y1={padT + innerH}
          y2={padT + innerH}
          stroke="var(--rule)"
          strokeWidth={1}
        />

        {/* bars */}
        {view.bins.map((b, i) => {
          const x = xScale(b.x0);
          const w = Math.max(1, xScale(b.x1) - xScale(b.x0) - 1);
          const y = yScale(b.count);
          const h = padT + innerH - y;
          return (
            <g key={i}>
              <rect x={x} y={y} width={w} height={h} fill={color} opacity={0.85} />
              {b.count > 0 && (
                <title>
                  {b.x0.toFixed(2)} – {b.x1.toFixed(2)} : {b.count}
                </title>
              )}
            </g>
          );
        })}

        {/* mean marker */}
        {Number.isFinite(view.mean) && (
          <line
            x1={xScale(view.mean)}
            x2={xScale(view.mean)}
            y1={padT}
            y2={padT + innerH}
            stroke="var(--syn-kw)"
            strokeWidth={1}
            strokeDasharray="3 3"
          />
        )}
        {/* median marker */}
        {Number.isFinite(view.median) && view.median !== view.mean && (
          <line
            x1={xScale(view.median)}
            x2={xScale(view.median)}
            y1={padT}
            y2={padT + innerH}
            stroke="var(--syn-str)"
            strokeWidth={1}
            strokeDasharray="3 3"
          />
        )}

        {/* tick labels: min, max */}
        <text
          x={padL}
          y={H - 6}
          fontFamily="var(--mono)"
          fontSize={11}
          fill="var(--text-faint)"
        >
          {formatTick(view.bins[0].x0)}
        </text>
        <text
          x={W - padR}
          y={H - 6}
          fontFamily="var(--mono)"
          fontSize={11}
          fill="var(--text-faint)"
          textAnchor="end"
        >
          {formatTick(view.bins.at(-1)!.x1)}
        </text>

        {/* count tick: max */}
        <text
          x={padL - 4}
          y={padT + 4}
          fontFamily="var(--mono)"
          fontSize={11}
          fill="var(--text-faint)"
          textAnchor="end"
        >
          {view.maxCount}
        </text>
      </svg>
      {caption && (
        <figcaption
          style={{
            fontFamily: "var(--mono)",
            fontSize: 11,
            color: "var(--text-muted)",
            marginTop: 4,
          }}
        >
          {caption}
        </figcaption>
      )}
    </figure>
  );
}

function formatTick(v: number): string {
  if (Number.isInteger(v)) return String(v);
  if (Math.abs(v) >= 100) return v.toFixed(0);
  if (Math.abs(v) >= 10) return v.toFixed(1);
  return v.toFixed(2);
}
