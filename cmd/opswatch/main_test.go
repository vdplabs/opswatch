package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vdplabs/opswatch/internal/domain"
	"github.com/vdplabs/opswatch/internal/vision"
)

func TestFilterAlertCooldown(t *testing.T) {
	alert := domain.Alert{
		Severity: domain.SeverityCritical,
		Title:    "Possible DNS intent mismatch",
		Evidence: []string{"observed: Create hosted zone"},
	}
	lastAlertAt := make(map[string]time.Time)
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	first := filterAlertCooldown([]domain.Alert{alert}, lastAlertAt, time.Minute, now)
	if len(first) != 1 {
		t.Fatalf("expected first alert through, got %d", len(first))
	}

	second := filterAlertCooldown([]domain.Alert{alert}, lastAlertAt, time.Minute, now.Add(30*time.Second))
	if len(second) != 0 {
		t.Fatalf("expected duplicate alert suppressed, got %d", len(second))
	}

	third := filterAlertCooldown([]domain.Alert{alert}, lastAlertAt, time.Minute, now.Add(2*time.Minute))
	if len(third) != 1 {
		t.Fatalf("expected alert after cooldown, got %d", len(third))
	}
}

func TestRearmAlertCooldownAllowsReturnOfResolvedAlert(t *testing.T) {
	alert := domain.Alert{
		Severity: domain.SeverityCritical,
		Title:    "Protected domain zone creation",
		Evidence: []string{"observed: Create hosted zone for example.com"},
	}
	lastAlertAt := make(map[string]time.Time)
	active := make(map[string]bool)
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	rearmAlertCooldown([]domain.Alert{alert}, lastAlertAt, active)
	first := filterAlertCooldown([]domain.Alert{alert}, lastAlertAt, time.Hour, now)
	if len(first) != 1 {
		t.Fatalf("expected first alert through, got %d", len(first))
	}

	rearmAlertCooldown(nil, lastAlertAt, active)
	if len(active) != 0 {
		t.Fatalf("expected active signatures cleared, got %#v", active)
	}

	rearmAlertCooldown([]domain.Alert{alert}, lastAlertAt, active)
	second := filterAlertCooldown([]domain.Alert{alert}, lastAlertAt, time.Hour, now.Add(10*time.Second))
	if len(second) != 1 {
		t.Fatalf("expected alert to fire again after disappearing, got %d", len(second))
	}
}

func TestParseCaptureRect(t *testing.T) {
	rect, ok, err := parseCaptureRect("600,0,1440,1000")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rect to be present")
	}
	if rect.X != 600 || rect.Y != 0 || rect.Width != 1440 || rect.Height != 1000 {
		t.Fatalf("unexpected rect: %#v", rect)
	}
}

func TestParseCaptureRectEmpty(t *testing.T) {
	_, ok, err := parseCaptureRect("")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no rect")
	}
}

func TestContextInitCreatesPack(t *testing.T) {
	dir := t.TempDir()
	if err := runContext(context.Background(), []string{"init", "--context-dir", dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "company.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestEnrichFrameWithContext(t *testing.T) {
	events := []domain.Event{{
		Source: domain.SourceRunbook,
		Context: map[string]string{
			"intent":          "Add DNS record",
			"expected_action": "add CNAME",
			"environment":     "prod",
		},
	}, {
		Source: domain.SourceAPI,
		Context: map[string]string{
			"kind":   "protected_domain",
			"domain": "example.com",
		},
	}}

	frame := enrichFrameWithContext(visionFrame(), events)
	if frame.Intent != "Add DNS record" || frame.ExpectedAction != "add CNAME" || frame.Environment != "prod" {
		t.Fatalf("frame was not enriched: %#v", frame)
	}
	if len(frame.ProtectedDomains) != 1 || frame.ProtectedDomains[0] != "example.com" {
		t.Fatalf("expected protected domain, got %#v", frame.ProtectedDomains)
	}
}

func TestNotificationMessageIncludesObservedAndIntent(t *testing.T) {
	alert := domain.Alert{
		Explanation: "Observed DNS zone creation while current intent appears to be adding or changing a DNS record.",
		Evidence: []string{
			"intent: add a DNS record",
			"observed: create a new primary DNS zone",
		},
	}

	got := notificationMessage(alert)
	want := "Observed: create a new primary DNS zone | Intent: add a DNS record"
	if got != want {
		t.Fatalf("unexpected notification message %q", got)
	}
}

func TestNotificationMessageFallsBackToExplanation(t *testing.T) {
	alert := domain.Alert{Explanation: "generic explanation"}
	if got := notificationMessage(alert); got != "generic explanation" {
		t.Fatalf("unexpected fallback message %q", got)
	}
}

func TestNormalizeOCREventCorrectsProtectedDomainTypo(t *testing.T) {
	event := domain.Event{
		Text: "Create hosted zone for exatnple.com",
		Context: map[string]string{
			"domain": "exatnple.com",
		},
	}
	frame := vision.FrameContext{ProtectedDomains: []string{"example.com"}}
	got := normalizeOCREvent(event, frame)
	if got.Context["domain"] != "example.com" {
		t.Fatalf("expected corrected domain, got %q", got.Context["domain"])
	}
	if got.Text != "Create hosted zone for example.com" {
		t.Fatalf("unexpected normalized text %q", got.Text)
	}
}

func visionFrame() vision.FrameContext {
	return vision.FrameContext{Actor: "local-operator"}
}
