package doctor

import "testing"

func TestHasFailures(t *testing.T) {
	if HasFailures([]Check{{Status: StatusOK}, {Status: StatusWarn}}) {
		t.Fatal("did not expect failures")
	}
	if !HasFailures([]Check{{Status: StatusFail}}) {
		t.Fatal("expected failures")
	}
}
