package doctor

import (
	"context"
	"testing"
)

func TestHasFailures(t *testing.T) {
	if HasFailures([]Check{{Status: StatusOK}, {Status: StatusWarn}}) {
		t.Fatal("did not expect failures")
	}
	if !HasFailures([]Check{{Status: StatusFail}}) {
		t.Fatal("expected failures")
	}
}

func TestRunSkipsSourceCheckoutChecksWithoutRepoRoot(t *testing.T) {
	checks := Run(context.Background(), Options{VisionProvider: "unsupported"})
	for _, check := range checks {
		if check.Name == "repo root" || check.Name == "go" {
			t.Fatalf("did not expect source checkout check without repo root, got %q", check.Name)
		}
	}
}
