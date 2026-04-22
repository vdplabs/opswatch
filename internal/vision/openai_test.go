package vision

import (
	"testing"

	"github.com/vdplabs/opswatch/internal/domain"
)

func TestExtractOutputTextAndParseVisionEvent(t *testing.T) {
	body := []byte(`{
		"output": [{
			"content": [{
				"type": "output_text",
				"text": "{\"source\":\"screen\",\"text\":\"AWS Route53 Create hosted zone example.com\",\"context\":{\"action\":\"create\",\"resource_type\":\"hosted_zone\",\"domain\":\"example.com\",\"environment\":\"prod\"}}"
			}]
		}]
	}`)

	text, err := extractOutputText(body)
	if err != nil {
		t.Fatal(err)
	}

	event, err := parseVisionEvent(text)
	if err != nil {
		t.Fatal(err)
	}
	if event.Source != domain.SourceScreen {
		t.Fatalf("expected screen source, got %q", event.Source)
	}
	if event.Context["resource_type"] != "hosted_zone" {
		t.Fatalf("expected hosted_zone resource, got %q", event.Context["resource_type"])
	}
}

func TestParseVisionEventRepairsTruncatedJSON(t *testing.T) {
	text := "{\n" +
		"  \"source\":\"screen\",\n" +
		"  \"text\":\"kubectl delete deployment\",\n" +
		"  \"context\":{\n" +
		"    \"action\":\"delete\",\n" +
		"    \"resource_type\":\"deployment\",\n" +
		"    \"environment\":\"prod\"\n"

	event, err := parseVisionEvent(text)
	if err != nil {
		t.Fatal(err)
	}
	if event.Text != "kubectl delete deployment" {
		t.Fatalf("unexpected text %q", event.Text)
	}
	if event.Context["action"] != "delete" {
		t.Fatalf("unexpected action %q", event.Context["action"])
	}
	if event.Context["environment"] != "prod" {
		t.Fatalf("unexpected environment %q", event.Context["environment"])
	}
}
