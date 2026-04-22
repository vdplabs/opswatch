package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
}

type Options struct {
	VisionProvider string
	Model          string
	OllamaEndpoint string
	RepoRoot       string
}

func Run(ctx context.Context, options Options) []Check {
	var checks []Check
	if options.RepoRoot != "" {
		checks = append(checks, checkRepoRoot(options.RepoRoot))
		checks = append(checks, checkExecutable("go"))
	}

	if runtime.GOOS == "darwin" {
		checks = append(checks, checkExecutable("screencapture"))
		checks = append(checks, checkExecutable("sips"))
		checks = append(checks, checkExecutable("osascript"))
	}

	switch strings.ToLower(strings.TrimSpace(options.VisionProvider)) {
	case "", "ollama":
		checks = append(checks, checkOllama(ctx, options))
	case "openai":
		checks = append(checks, checkOpenAI())
	default:
		checks = append(checks, Check{
			Name:    "vision provider",
			Status:  StatusFail,
			Message: "unsupported provider " + options.VisionProvider,
		})
	}

	return checks
}

func HasFailures(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

func checkRepoRoot(root string) Check {
	if _, err := os.Stat(root + "/go.mod"); err != nil {
		return Check{Name: "repo root", Status: StatusFail, Message: "go.mod not found under " + root}
	}
	return Check{Name: "repo root", Status: StatusOK, Message: root}
}

func checkExecutable(name string) Check {
	path, err := exec.LookPath(name)
	if err != nil {
		return Check{Name: name, Status: StatusFail, Message: name + " not found in PATH"}
	}
	return Check{Name: name, Status: StatusOK, Message: path}
}

func checkOpenAI() Check {
	if os.Getenv("OPENAI_API_KEY") == "" {
		return Check{Name: "openai api key", Status: StatusFail, Message: "OPENAI_API_KEY is not set"}
	}
	return Check{Name: "openai api key", Status: StatusOK, Message: "OPENAI_API_KEY is set"}
}

func checkOllama(ctx context.Context, options Options) Check {
	endpoint := options.OllamaEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434/api/tags"
	} else {
		endpoint = strings.TrimSuffix(endpoint, "/api/generate") + "/api/tags"
	}
	model := options.Model
	if model == "" {
		model = "qwen2.5vl:3b-q4_K_M"
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Check{Name: "ollama", Status: StatusFail, Message: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Check{Name: "ollama", Status: StatusFail, Message: "cannot reach Ollama at " + endpoint + ": " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Check{Name: "ollama", Status: StatusFail, Message: fmt.Sprintf("Ollama returned %s", resp.Status)}
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return Check{Name: "ollama", Status: StatusFail, Message: "could not parse Ollama tags: " + err.Error()}
	}
	for _, item := range tags.Models {
		if item.Name == model || strings.TrimSuffix(item.Name, ":latest") == model {
			return Check{Name: "ollama", Status: StatusOK, Message: "model available: " + item.Name}
		}
	}
	return Check{Name: "ollama", Status: StatusFail, Message: "model not found: run `ollama pull " + model + "`"}
}
