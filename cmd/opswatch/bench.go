package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/vdplabs/opswatch/internal/analyzer"
	"github.com/vdplabs/opswatch/internal/contextpack"
	"github.com/vdplabs/opswatch/internal/domain"
	"github.com/vdplabs/opswatch/internal/policy"
	"github.com/vdplabs/opswatch/internal/vision"
)

func runBench(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("bench command requires a target, currently: vision")
	}
	switch args[0] {
	case "vision":
		return runBenchVision(ctx, args[1:])
	default:
		return fmt.Errorf("unsupported bench target %q", args[0])
	}
}

func runBenchVision(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("bench vision", flag.ContinueOnError)
	imagePath := fs.String("image", "", "path to screenshot/image")
	modelsValue := fs.String("models", "qwen2.5vl:3b-q4_K_M,qwen2.5vl,granite3.2-vision,llama3.2-vision", "comma-separated vision models")
	visionProvider := fs.String("vision-provider", "ollama", "vision provider: ollama or openai")
	contextDir := fs.String("context-dir", defaultContextDir(), "directory or file containing local context packs; set empty to disable")
	runs := fs.Int("runs", 3, "benchmark runs per model")
	environment := fs.String("environment", "", "known environment, such as prod")
	intent := fs.String("intent", "", "current stated operator intent")
	expectedAction := fs.String("expected-action", "", "expected runbook action")
	protectedDomains := fs.String("protected-domain", "", "comma-separated protected domains")
	ollamaEndpoint := fs.String("ollama-endpoint", "", "Ollama generate endpoint")
	visionTimeout := fs.Duration("vision-timeout", 5*time.Minute, "per-image vision analysis timeout")
	maxImageDimension := fs.Int("max-image-dimension", 1000, "resize image to this max dimension before benchmarking; 0 disables")
	ollamaNumPredict := fs.Int("ollama-num-predict", 128, "maximum Ollama output tokens")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *imagePath == "" {
		return fmt.Errorf("--image is required")
	}
	if *runs <= 0 {
		return fmt.Errorf("--runs must be greater than zero")
	}
	models := splitCSV(*modelsValue)
	if len(models) == 0 {
		return fmt.Errorf("--models must include at least one model")
	}

	benchImagePath := *imagePath
	if *maxImageDimension > 0 {
		resizedPath, err := resizedImagePath(ctx, *imagePath, *maxImageDimension)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opswatch: warning: failed to resize benchmark image: %v\n", err)
		} else {
			benchImagePath = resizedPath
			defer cleanupFrame(resizedPath, false)
		}
	}

	contextEvents, err := contextpack.LoadDir(ctx, *contextDir)
	if err != nil {
		return err
	}
	frame := enrichFrameWithContext(vision.FrameContext{
		Intent:           *intent,
		ExpectedAction:   *expectedAction,
		Environment:      *environment,
		ProtectedDomains: splitCSV(*protectedDomains),
		Actor:            "local-operator",
	}, contextEvents)

	results := make([]visionBenchResult, 0, len(models))
	for _, model := range models {
		result := benchmarkVisionModel(ctx, benchImagePath, model, *visionProvider, *ollamaEndpoint, *visionTimeout, *ollamaNumPredict, *runs, frame, contextEvents)
		results = append(results, result)
	}

	switch *format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "text":
		writeVisionBenchText(results)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
}

type visionBenchResult struct {
	Model       string `json:"model"`
	Runs        int    `json:"runs"`
	Successes   int    `json:"successes"`
	Failures    int    `json:"failures"`
	AverageMS   int64  `json:"average_ms"`
	P95MS       int64  `json:"p95_ms"`
	MinMS       int64  `json:"min_ms"`
	MaxMS       int64  `json:"max_ms"`
	LastAlerts  int    `json:"last_alerts"`
	LastEvent   string `json:"last_event,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	JSONOKRatio string `json:"json_ok_ratio"`
}

func benchmarkVisionModel(ctx context.Context, imagePath, model, provider, ollamaEndpoint string, timeout time.Duration, numPredict, runs int, frame vision.FrameContext, contextEvents []domain.Event) visionBenchResult {
	result := visionBenchResult{
		Model: model,
		Runs:  runs,
	}
	durations := make([]time.Duration, 0, runs)

	for i := 0; i < runs; i++ {
		started := time.Now()
		events, _, err := imageEvents(ctx, imagePath, frame, visionOptions{
			Provider:         provider,
			Model:            model,
			OllamaEndpoint:   ollamaEndpoint,
			Timeout:          timeout,
			OllamaNumPredict: numPredict,
		})
		duration := time.Since(started)
		if err != nil {
			result.Failures++
			result.LastError = err.Error()
			continue
		}

		durations = append(durations, duration)
		result.Successes++
		result.LastError = ""
		if len(events) > 0 {
			last := events[len(events)-1]
			result.LastEvent = last.Text
		}

		engine := analyzer.New(policy.DefaultPolicies())
		alerts, err := engine.AnalyzeEvents(ctx, withContextEvents(contextEvents, events))
		if err != nil {
			result.Failures++
			result.Successes--
			result.LastError = err.Error()
			continue
		}
		result.LastAlerts = len(alerts)
	}

	result.JSONOKRatio = fmt.Sprintf("%d/%d", result.Successes, result.Runs)
	result.AverageMS = durationAverageMS(durations)
	result.P95MS = durationPercentileMS(durations, 0.95)
	result.MinMS = durationMinMS(durations)
	result.MaxMS = durationMaxMS(durations)
	return result
}

func writeVisionBenchText(results []visionBenchResult) {
	fmt.Fprintf(os.Stdout, "%-24s %6s %8s %8s %8s %8s %8s %7s %s\n", "model", "json", "avg", "p95", "min", "max", "alerts", "fail", "last")
	for _, result := range results {
		last := result.LastEvent
		if result.LastError != "" {
			last = result.LastError
		}
		fmt.Fprintf(
			os.Stdout,
			"%-24s %6s %7dms %7dms %7dms %7dms %8d %7d %s\n",
			result.Model,
			result.JSONOKRatio,
			result.AverageMS,
			result.P95MS,
			result.MinMS,
			result.MaxMS,
			result.LastAlerts,
			result.Failures,
			truncateForNotification(last, 96),
		)
	}
}

func durationAverageMS(values []time.Duration) int64 {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total.Milliseconds() / int64(len(values))
}

func durationPercentileMS(values []time.Duration, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index].Milliseconds()
}

func durationMinMS(values []time.Duration) int64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min.Milliseconds()
}

func durationMaxMS(values []time.Duration) int64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max.Milliseconds()
}
