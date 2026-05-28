import { useMemo } from "react";
import { max as amax, min as amin } from "../lib/stats";

export type Series = {
  id: number;
  label: string;
  color: string;
  values: number[];
};

type Props = {
  series: Series[];
  binCount?: number;
  height?: number;
};

// OverlayHistogram draws one frequency polygon per series over a shared set of
// bins. Counts are normalised to density (fraction of that series' runs per
// bin) so distributions with different sample sizes — e.g. a 100-seed vs a
// 500-seed suite — are directly comparable. Mean markers sit along the top.
export function OverlayHistogram({ series, binCount = 24, height = 240 }: Props) {
  const view = useMemo(() => {
    const withData = series.filter((s) => s.values.length > 0);
    if (withData.length === 0) return null;
    const all = withData.flatMap((s) => s.values);
    const lo = amin(all);
    const hi = amax(all);
    if (!(hi > lo)) return { degenerate: true as const, value: lo };

    const width = (hi - lo) / binCount;
    const centers = Array.from(
      { length: binCount },
      (_, i) => lo + (i + 0.5) * width,
    );

    const lines = withData.map((s) => {
      const counts = new Array(binCount).fill(0);
      for (const v of s.values) {
        let idx = Math.floor((v - lo) / width);
        if (idx >= binCount) idx = binCount - 1;
        if (idx < 0) idx = 0;
        counts[idx]++;
      }
      const density = counts.map((c) => c / s.values.length);
      const m = s.values.reduce((a, b) => a + b, 0) / s.values.length;
      return { series: s, density, mean: m };
    });

    const yMax = Math.max(
      ...lines.flatMap((l) => l.density),
      Number.EPSILON,
    );
    return { degenerate: false as const, lo, hi, centers, lines, yMax };
  }, [series, binCount]);

  if (!view) return <p className="t-cmt">no data to compare</p>;
  if (view.degenerate) {
    return (
      <p className="t-cmt">
        all selected scores are identical ({formatTick(view.value)}) — nothing
        to plot
      </p>
    );
  }

  const W = 800;
  const H = height;
  const padL = 30;
  const padR = 10;
  const padT = 10;
  const padB = 24;
  const innerW = W - padL - padR;
  const innerH = H - padT - padB;

  const xScale = (v: number) =>
    padL + ((v - view.lo) / (view.hi - view.lo)) * innerW;
  const yScale = (d: number) => padT + innerH - (d / view.yMax) * innerH;

  return (
    <figure style={{ margin: 0 }}>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} role="img" aria-label="overlaid score distributions">
        <line
          x1={padL}
          x2={W - padR}
          y1={padT + innerH}
          y2={padT + innerH}
          stroke="var(--rule)"
          strokeWidth={1}
        />

        {view.lines.map((l) => {
          const pts = l.density
            .map((d, i) => `${xScale(view.centers[i])},${yScale(d)}`)
            .join(" ");
          return (
            <g key={l.series.id}>
              <polyline
                points={pts}
                fill="none"
                stroke={l.series.color}
                strokeWidth={2}
                strokeLinejoin="round"
                opacity={0.9}
              />
              <line
                x1={xScale(l.mean)}
                x2={xScale(l.mean)}
                y1={padT}
                y2={padT + innerH}
                stroke={l.series.color}
                strokeWidth={1}
                strokeDasharray="2 3"
                opacity={0.6}
              />
            </g>
          );
        })}

        <text x={padL} y={H - 6} fontFamily="var(--mono)" fontSize={11} fill="var(--text-faint)">
          {formatTick(view.lo)}
        </text>
        <text
          x={W - padR}
          y={H - 6}
          fontFamily="var(--mono)"
          fontSize={11}
          fill="var(--text-faint)"
          textAnchor="end"
        >
          {formatTick(view.hi)}
        </text>
      </svg>
      <figcaption
        style={{
          fontFamily: "var(--mono)",
          fontSize: 11,
          color: "var(--text-muted)",
          marginTop: 4,
        }}
      >
        density (fraction of runs per bin) · {binCount} bins · dashed line = mean
      </figcaption>
    </figure>
  );
}

function formatTick(v: number): string {
  if (Number.isInteger(v)) return String(v);
  if (Math.abs(v) >= 100) return v.toFixed(0);
  if (Math.abs(v) >= 10) return v.toFixed(1);
  return v.toFixed(2);
}
