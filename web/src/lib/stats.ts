// Stats helpers used by SubmissionDetail's per-suite breakdown.
// All functions return NaN for empty input — caller is responsible for
// guarding before display.

export function mean(xs: number[]): number {
  if (xs.length === 0) return NaN;
  return xs.reduce((a, b) => a + b, 0) / xs.length;
}

export function stddev(xs: number[]): number {
  if (xs.length < 2) return NaN;
  const m = mean(xs);
  const variance =
    xs.reduce((acc, v) => acc + (v - m) ** 2, 0) / (xs.length - 1);
  return Math.sqrt(variance);
}

export function min(xs: number[]): number {
  return xs.length === 0 ? NaN : Math.min(...xs);
}

export function max(xs: number[]): number {
  return xs.length === 0 ? NaN : Math.max(...xs);
}

// percentile uses linear interpolation between the two nearest ranks (the
// "C = 1" convention also known as the default in numpy/excel).
export function percentile(xs: number[], p: number): number {
  if (xs.length === 0) return NaN;
  if (xs.length === 1) return xs[0];
  const sorted = [...xs].sort((a, b) => a - b);
  const rank = (p / 100) * (sorted.length - 1);
  const lo = Math.floor(rank);
  const hi = Math.ceil(rank);
  if (lo === hi) return sorted[lo];
  return sorted[lo] + (sorted[hi] - sorted[lo]) * (rank - lo);
}

export function median(xs: number[]): number {
  return percentile(xs, 50);
}

// histogram returns evenly-spaced bins between min and max with the count
// of values falling into each. Edge value goes to the last bin.
export function histogram(
  xs: number[],
  binCount = 12,
): { x0: number; x1: number; count: number }[] {
  if (xs.length === 0 || binCount < 1) return [];
  const lo = min(xs);
  const hi = max(xs);
  if (lo === hi) {
    return [{ x0: lo, x1: lo, count: xs.length }];
  }
  const width = (hi - lo) / binCount;
  const bins = Array.from({ length: binCount }, (_, i) => ({
    x0: lo + i * width,
    x1: lo + (i + 1) * width,
    count: 0,
  }));
  for (const v of xs) {
    let idx = Math.floor((v - lo) / width);
    if (idx >= binCount) idx = binCount - 1; // top edge falls into last bin
    if (idx < 0) idx = 0;
    bins[idx].count++;
  }
  return bins;
}
