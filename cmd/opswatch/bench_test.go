package main

import (
	"testing"
	"time"
)

func TestDurationSummary(t *testing.T) {
	values := []time.Duration{
		100 * time.Millisecond,
		300 * time.Millisecond,
		200 * time.Millisecond,
	}

	if got := durationAverageMS(values); got != 200 {
		t.Fatalf("average = %d, want 200", got)
	}
	if got := durationMinMS(values); got != 100 {
		t.Fatalf("min = %d, want 100", got)
	}
	if got := durationMaxMS(values); got != 300 {
		t.Fatalf("max = %d, want 300", got)
	}
	if got := durationPercentileMS(values, 0.95); got != 300 {
		t.Fatalf("p95 = %d, want 300 for nearest-rank over three values", got)
	}
}

func TestDurationSummaryEmpty(t *testing.T) {
	if durationAverageMS(nil) != 0 || durationMinMS(nil) != 0 || durationMaxMS(nil) != 0 || durationPercentileMS(nil, 0.95) != 0 {
		t.Fatal("expected empty summaries to be zero")
	}
}
