package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vdplabs/opswatch/internal/domain"
)

const defaultOllamaEndpoint = "http://localhost:11434/api/generate"

type OllamaClient struct {
	Model      string
	Endpoint   string
	Timeout    time.Duration
	Options    map[string]any
	HTTPClient *http.Client
}

func NewOllamaClient(model string, endpoint string, timeout time.Duration) *OllamaClient {
	if model == "" {
		model = "qwen2.5vl:3b-q4_K_M"
	}
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &OllamaClient{
		Model:    model,
		Endpoint: endpoint,
		Timeout:  timeout,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OllamaClient) AnalyzeImage(ctx context.Context, imagePath string, frame FrameContext) (domain.Event, error) {
	encoded, _, err := imageBase64(imagePath)
	if err != nil {
		return domain.Event{}, err
	}

	reqBody := ollamaGenerateRequest{
		Model:  c.Model,
		Prompt: buildPrompt(frame),
		Images: []string{encoded},
		Stream: false,
		Format: "json",
		Options: map[string]any{
			"temperature": 0,
			"num_predict": 256,
			"num_ctx":     4096,
		},
	}
	for key, value := range c.Options {
		reqBody.Options[key] = value
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return domain.Event{}, err
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return domain.Event{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return domain.Event{}, fmt.Errorf("ollama vision request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.Event{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.Event{}, fmt.Errorf("ollama vision request failed: %s: %s", resp.Status, string(body))
	}

	var response ollamaGenerateResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.Event{}, err
	}
	if strings.TrimSpace(response.Response) == "" {
		return domain.Event{}, fmt.Errorf("ollama response did not contain generated text")
	}

	event, err := parseVisionEvent(response.Response)
	if err != nil {
		return domain.Event{}, err
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Source == "" {
		event.Source = domain.SourceScreen
	}
	if event.Actor == "" {
		event.Actor = frame.Actor
	}
	if event.Context == nil {
		event.Context = make(map[string]string)
	}
	event.Context["image_path"] = imagePath
	event.Context["vision_provider"] = "ollama"
	if frame.Environment != "" && event.Context["environment"] == "" {
		event.Context["environment"] = frame.Environment
	}
	return event, nil
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Images  []string       `json:"images"`
	Stream  bool           `json:"stream"`
	Format  string         `json:"format"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}
