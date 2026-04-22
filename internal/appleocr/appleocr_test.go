package appleocr

import (
	"testing"

	"github.com/vdplabs/opswatch/internal/domain"
)

func TestIsMeaningfulRejectsGenericLowRiskOCR(t *testing.T) {
	event := domain.Event{
		Text: "Some generic text from a screen",
		Context: map[string]string{
			"risk_hint": "low",
		},
	}
	if isMeaningful(event) {
		t.Fatal("expected generic low-risk OCR event to be ignored")
	}
}

func TestIsMeaningfulAcceptsStructuredAction(t *testing.T) {
	event := domain.Event{
		Text: "Create hosted zone",
		Context: map[string]string{
			"action":        "create",
			"resource_type": "hosted_zone",
			"risk_hint":     "high",
		},
	}
	if !isMeaningful(event) {
		t.Fatal("expected structured OCR event to be accepted")
	}
}
