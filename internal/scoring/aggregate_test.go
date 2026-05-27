package scoring

import (
	"math"
	"testing"
)

func TestAggregateStrategies(t *testing.T) {
	scores := []float64{1, 5, 2, 9, 3}
	cases := []struct {
		strategy Strategy
		want     float64
	}{
		{Mean, 4.0},
		{Median, 3.0},
		{Max, 9.0},
		{Min, 1.0},
	}
	for _, c := range cases {
		t.Run(string(c.strategy), func(t *testing.T) {
			got := Aggregate(scores, c.strategy)
			if got != c.want {
				t.Errorf("Aggregate(%v) = %v, want %v", c.strategy, got, c.want)
			}
		})
	}
}

func TestEvenMedian(t *testing.T) {
	if got := Aggregate([]float64{1, 2, 3, 4}, Median); got != 2.5 {
		t.Errorf("median of [1,2,3,4] = %v, want 2.5", got)
	}
}

func TestMode(t *testing.T) {
	cases := []struct {
		name   string
		scores []float64
		want   float64
	}{
		{"clear winner", []float64{1, 2, 2, 3, 2}, 2},
		{"tied, picks lower", []float64{1, 1, 2, 2}, 1},
		{"all unique, picks lowest", []float64{3, 5, 1, 7}, 1},
		{"single", []float64{42}, 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Aggregate(c.scores, Mode)
			if got != c.want {
				t.Errorf("Mode(%v) = %v, want %v", c.scores, got, c.want)
			}
		})
	}
}

func TestEmptyIsNaN(t *testing.T) {
	if got := Aggregate(nil, Mean); !math.IsNaN(got) {
		t.Errorf("empty Aggregate = %v, want NaN", got)
	}
}

func TestFromConfigDefaultsToMean(t *testing.T) {
	if got := FromConfig(nil); got != Mean {
		t.Errorf("FromConfig(nil) = %v, want %v", got, Mean)
	}
	if got := FromConfig(map[string]any{"aggregate": "garbage"}); got != Mean {
		t.Errorf("garbage aggregate should fall back to Mean, got %v", got)
	}
	for _, name := range []Strategy{Mean, Median, Mode, Max, Min} {
		got := FromConfig(map[string]any{"aggregate": string(name)})
		if got != name {
			t.Errorf("FromConfig(%q) = %v, want %v", name, got, name)
		}
	}
}
