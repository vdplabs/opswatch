package terminalscrape

import "testing"

func TestSupportedApp(t *testing.T) {
	for _, owner := range []string{"Terminal", "iTerm2", "iTerm"} {
		if !SupportedApp(owner) {
			t.Fatalf("expected %q to be supported", owner)
		}
	}
	if SupportedApp("Safari") {
		t.Fatal("did not expect Safari to be supported")
	}
}

func TestExtractCommandFindsLatestInfraCommand(t *testing.T) {
	content := `
last login: Mon Apr 20
vishal@mbp ~ % echo hello
hello
vishal@mbp ~ % kubectl get pods
NAME READY STATUS
vishal@mbp ~ % kubectl delete deployment $DEPLOYMENT_ID
deployment.apps "api" deleted
`
	got := extractCommand(content)
	want := "kubectl delete deployment $DEPLOYMENT_ID"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExtractCommandReturnsEmptyWhenNoCommandPresent(t *testing.T) {
	if got := extractCommand("plain output only\nno shell command here"); got != "" {
		t.Fatalf("expected empty command, got %q", got)
	}
}
