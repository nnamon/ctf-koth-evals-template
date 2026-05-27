// Package scoring aggregates per-run scores into a single suite-level value.
// The strategy is selected by the suite's scoring.aggregate field.
package scoring

import (
	"math"
	"sort"
)

// Strategy is the aggregation function name. Default Mean.
type Strategy string

const (
	Mean   Strategy = "mean"
	Median Strategy = "median"
	Mode   Strategy = "mode"
	Max    Strategy = "max"
	Min    Strategy = "min"
	Stddev Strategy = "stddev"
)

// FromConfig pulls the strategy from a suite's scoring config map. Falls
// back to Mean when missing or unrecognised.
func FromConfig(cfg map[string]any) Strategy {
	v, _ := cfg["aggregate"].(string)
	switch Strategy(v) {
	case Mean, Median, Mode, Max, Min, Stddev:
		return Strategy(v)
	}
	return Mean
}

// Aggregate returns the aggregated score, or NaN if scores is empty.
func Aggregate(scores []float64, s Strategy) float64 {
	if len(scores) == 0 {
		return math.NaN()
	}
	switch s {
	case Median:
		cp := append([]float64(nil), scores...)
		sort.Float64s(cp)
		mid := len(cp) / 2
		if len(cp)%2 == 1 {
			return cp[mid]
		}
		return (cp[mid-1] + cp[mid]) / 2
	case Mode:
		counts := map[float64]int{}
		for _, v := range scores {
			counts[v]++
		}
		best := scores[0]
		bestCount := 0
		for v, c := range counts {
			if c > bestCount || (c == bestCount && v < best) {
				best = v
				bestCount = c
			}
		}
		return best
	case Max:
		m := scores[0]
		for _, v := range scores[1:] {
			if v > m {
				m = v
			}
		}
		return m
	case Min:
		m := scores[0]
		for _, v := range scores[1:] {
			if v < m {
				m = v
			}
		}
		return m
	case Stddev:
		if len(scores) < 2 {
			return 0
		}
		var t float64
		for _, v := range scores {
			t += v
		}
		m := t / float64(len(scores))
		var sq float64
		for _, v := range scores {
			sq += (v - m) * (v - m)
		}
		return math.Sqrt(sq / float64(len(scores)-1))
	case Mean:
		fallthrough
	default:
		var t float64
		for _, v := range scores {
			t += v
		}
		return t / float64(len(scores))
	}
}
