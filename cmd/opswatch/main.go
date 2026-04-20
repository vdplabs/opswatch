package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vdplabs/opswatch/internal/analyzer"
	"github.com/vdplabs/opswatch/internal/capture"
	"github.com/vdplabs/opswatch/internal/doctor"
	"github.com/vdplabs/opswatch/internal/domain"
	"github.com/vdplabs/opswatch/internal/framehash"
	"github.com/vdplabs/opswatch/internal/policy"
	"github.com/vdplabs/opswatch/internal/report"
	"github.com/vdplabs/opswatch/internal/vision"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "opswatch: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usage()
	}

	switch args[0] {
	case "analyze":
		return runAnalyze(ctx, args[1:])
	case "analyze-image":
		return runAnalyzeImage(ctx, args[1:])
	case "doctor":
		return runDoctor(ctx, args[1:])
	case "watch":
		return runWatch(ctx, args[1:])
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() error {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  opswatch analyze --events <events.jsonl>")
	fmt.Fprintln(os.Stderr, "  opswatch analyze-image --image <screenshot.png> [--vision-provider openai|ollama] [--intent <text>]")
	fmt.Fprintln(os.Stderr, "  opswatch doctor [--vision-provider openai|ollama]")
	fmt.Fprintln(os.Stderr, "  opswatch watch [--vision-provider openai|ollama] [--interval 2s] [--intent <text>]")
	return nil
}

func runDoctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	visionProvider := fs.String("vision-provider", "ollama", "vision provider: openai or ollama")
	model := fs.String("model", "llama3.2-vision", "vision model")
	ollamaEndpoint := fs.String("ollama-endpoint", "", "Ollama endpoint")
	repoRoot := fs.String("repo-root", ".", "OpsWatch repository root")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	checks := doctor.Run(ctx, doctor.Options{
		VisionProvider: *visionProvider,
		Model:          *model,
		OllamaEndpoint: *ollamaEndpoint,
		RepoRoot:       *repoRoot,
	})

	switch *format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(checks); err != nil {
			return err
		}
	case "text":
		for _, check := range checks {
			fmt.Fprintf(os.Stdout, "[%s] %s: %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Message)
		}
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
	if doctor.HasFailures(checks) {
		return fmt.Errorf("doctor found failing checks")
	}
	return nil
}

func runAnalyze(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	eventsPath := fs.String("events", "", "path to JSONL incident events")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *eventsPath == "" {
		return fmt.Errorf("--events is required")
	}

	file, err := os.Open(*eventsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	engine := analyzer.New(policy.DefaultPolicies())
	alerts, err := engine.AnalyzeJSONL(ctx, file)
	if err != nil {
		return err
	}

	switch *format {
	case "text":
		return report.WriteText(os.Stdout, alerts)
	case "json":
		return report.WriteJSON(os.Stdout, alerts)
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
}

func runAnalyzeImage(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("analyze-image", flag.ContinueOnError)
	imagePath := fs.String("image", "", "path to screenshot/image")
	intent := fs.String("intent", "", "current stated operator intent")
	expectedAction := fs.String("expected-action", "", "expected runbook action")
	environment := fs.String("environment", "", "known environment, such as prod")
	protectedDomains := fs.String("protected-domain", "", "comma-separated protected domains")
	visionProvider := fs.String("vision-provider", "openai", "vision provider: openai or ollama")
	model := fs.String("model", "", "vision model")
	ollamaEndpoint := fs.String("ollama-endpoint", "", "Ollama generate endpoint")
	visionTimeout := fs.Duration("vision-timeout", 5*time.Minute, "per-image vision analysis timeout")
	maxImageDimension := fs.Int("max-image-dimension", 1600, "resize image to this max dimension before analysis; 0 disables")
	ollamaNumPredict := fs.Int("ollama-num-predict", 256, "maximum Ollama output tokens")
	format := fs.String("format", "text", "output format: text or json")
	showEvents := fs.Bool("show-events", false, "print normalized events before alerts")
	saveEvents := fs.String("save-events", "", "write normalized events as JSONL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *imagePath == "" {
		return fmt.Errorf("--image is required")
	}
	imagePathForAnalysis := *imagePath
	if *maxImageDimension > 0 {
		resizedPath, err := resizedImagePath(ctx, *imagePath, *maxImageDimension)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opswatch: warning: failed to resize image: %v\n", err)
		} else {
			imagePathForAnalysis = resizedPath
		}
	}

	events, err := imageEvents(ctx, imagePathForAnalysis, vision.FrameContext{
		Intent:           *intent,
		ExpectedAction:   *expectedAction,
		Environment:      *environment,
		ProtectedDomains: splitCSV(*protectedDomains),
		Actor:            "local-operator",
	}, visionOptions{
		Provider:         *visionProvider,
		Model:            *model,
		OllamaEndpoint:   *ollamaEndpoint,
		Timeout:          *visionTimeout,
		OllamaNumPredict: *ollamaNumPredict,
	})
	if err != nil {
		return err
	}
	if *showEvents {
		if err := writeEventsJSONL(os.Stdout, events); err != nil {
			return err
		}
	}
	if *saveEvents != "" {
		if err := saveEventsJSONL(*saveEvents, events); err != nil {
			return err
		}
	}

	engine := analyzer.New(policy.DefaultPolicies())
	alerts, err := engine.AnalyzeEvents(ctx, events)
	if err != nil {
		return err
	}
	return writeAlerts(*format, alerts)
}

func saveEventsJSONL(path string, events []domain.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeEventsJSONL(file, events)
}

func writeEventsJSONL(w io.Writer, events []domain.Event) error {
	encoder := json.NewEncoder(w)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func runWatch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 2*time.Second, "capture interval")
	intent := fs.String("intent", "", "current stated operator intent")
	expectedAction := fs.String("expected-action", "", "expected runbook action")
	environment := fs.String("environment", "", "known environment, such as prod")
	protectedDomains := fs.String("protected-domain", "", "comma-separated protected domains")
	visionProvider := fs.String("vision-provider", "openai", "vision provider: openai or ollama")
	model := fs.String("model", "", "vision model")
	ollamaEndpoint := fs.String("ollama-endpoint", "", "Ollama generate endpoint")
	visionTimeout := fs.Duration("vision-timeout", 5*time.Minute, "per-frame vision analysis timeout")
	maxImageDimension := fs.Int("max-image-dimension", 1600, "resize captured frames to this max dimension before analysis; 0 disables")
	ollamaNumPredict := fs.Int("ollama-num-predict", 256, "maximum Ollama output tokens")
	captureDir := fs.String("capture-dir", filepath.Join(os.TempDir(), "opswatch-frames"), "directory for temporary captures")
	captureRectValue := fs.String("capture-rect", "", "capture rectangle as x,y,width,height instead of full screen")
	windowID := fs.Uint("window-id", 0, "macOS window id to capture instead of full screen")
	skipUnchanged := fs.Bool("skip-unchanged", true, "skip vision analysis when the frame looks unchanged")
	minAnalysisInterval := fs.Duration("min-analysis-interval", 30*time.Second, "minimum time between vision analyses for changed frames")
	changeThreshold := fs.Int("change-threshold", 4, "minimum visual hash distance needed to analyze a new frame")
	alertCooldown := fs.Duration("alert-cooldown", 2*time.Minute, "suppress repeated alerts with the same signature")
	notify := fs.Bool("notify", false, "show a local desktop notification when alerts are emitted")
	keepFrames := fs.Bool("keep-frames", false, "keep captured frame files for debugging")
	verbose := fs.Bool("verbose", false, "print per-frame timing diagnostics")
	once := fs.Bool("once", false, "capture and analyze one frame")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval <= 0 {
		return fmt.Errorf("--interval must be greater than zero")
	}
	if err := os.MkdirAll(*captureDir, 0o755); err != nil {
		return err
	}
	captureRect, hasCaptureRect, err := parseCaptureRect(*captureRectValue)
	if err != nil {
		return err
	}
	if *windowID != 0 && hasCaptureRect {
		return fmt.Errorf("--window-id and --capture-rect are mutually exclusive")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	frame := vision.FrameContext{
		Intent:           *intent,
		ExpectedAction:   *expectedAction,
		Environment:      *environment,
		ProtectedDomains: splitCSV(*protectedDomains),
		Actor:            "local-operator",
	}
	capturer := capture.MacOSCapture{}
	var lastHash framehash.Hash
	hasLastHash := false
	var lastAnalysisAt time.Time
	lastAlertAt := make(map[string]time.Time)

	for {
		frameStarted := time.Now()
		imagePath := filepath.Join(*captureDir, fmt.Sprintf("frame-%d.png", time.Now().UnixNano()))
		captureStarted := time.Now()
		if *windowID != 0 {
			err = capturer.Window(ctx, imagePath, uint32(*windowID))
		} else if hasCaptureRect {
			err = capturer.Rect(ctx, imagePath, captureRect)
		} else {
			err = capturer.Fullscreen(ctx, imagePath)
		}
		if err != nil {
			return err
		}
		captureDuration := time.Since(captureStarted)

		resizeStarted := time.Now()
		if err := capturer.ResizeMaxDimension(ctx, imagePath, *maxImageDimension); err != nil {
			fmt.Fprintf(os.Stderr, "opswatch: warning: failed to resize frame: %v\n", err)
		}
		resizeDuration := time.Since(resizeStarted)

		hashDuration := time.Duration(0)
		if *skipUnchanged {
			hashStarted := time.Now()
			hash, err := framehash.File(imagePath)
			hashDuration = time.Since(hashStarted)
			if err != nil {
				fmt.Fprintf(os.Stderr, "opswatch: warning: failed to hash frame: %v\n", err)
			} else if hasLastHash && framehash.Distance(lastHash, hash) < *changeThreshold {
				if *verbose {
					fmt.Fprintf(os.Stderr, "opswatch: frame skipped unchanged capture=%s resize=%s hash=%s total=%s\n", captureDuration.Round(time.Millisecond), resizeDuration.Round(time.Millisecond), hashDuration.Round(time.Millisecond), time.Since(frameStarted).Round(time.Millisecond))
				}
				cleanupFrame(imagePath, *keepFrames)
				if *once {
					return nil
				}
				waitForNextFrame(ctx, *interval)
				continue
			} else {
				lastHash = hash
				hasLastHash = true
			}
		}
		if !lastAnalysisAt.IsZero() && time.Since(lastAnalysisAt) < *minAnalysisInterval {
			if *verbose {
				fmt.Fprintf(os.Stderr, "opswatch: frame skipped throttle capture=%s resize=%s hash=%s since_last_analysis=%s total=%s\n", captureDuration.Round(time.Millisecond), resizeDuration.Round(time.Millisecond), hashDuration.Round(time.Millisecond), time.Since(lastAnalysisAt).Round(time.Millisecond), time.Since(frameStarted).Round(time.Millisecond))
			}
			cleanupFrame(imagePath, *keepFrames)
			if *once {
				return nil
			}
			waitForNextFrame(ctx, *interval)
			continue
		}

		visionStarted := time.Now()
		events, err := imageEvents(ctx, imagePath, frame, visionOptions{
			Provider:         *visionProvider,
			Model:            *model,
			OllamaEndpoint:   *ollamaEndpoint,
			Timeout:          *visionTimeout,
			OllamaNumPredict: *ollamaNumPredict,
		})
		visionDuration := time.Since(visionStarted)
		lastAnalysisAt = time.Now()
		if err != nil {
			fmt.Fprintf(os.Stderr, "opswatch: warning: frame analysis failed: %v\n", err)
			if *verbose {
				fmt.Fprintf(os.Stderr, "opswatch: frame failed capture=%s resize=%s hash=%s vision=%s total=%s\n", captureDuration.Round(time.Millisecond), resizeDuration.Round(time.Millisecond), hashDuration.Round(time.Millisecond), visionDuration.Round(time.Millisecond), time.Since(frameStarted).Round(time.Millisecond))
			}
			cleanupFrame(imagePath, *keepFrames)
			if *once {
				return err
			}
			waitForNextFrame(ctx, *interval)
			continue
		}
		engine := analyzer.New(policy.DefaultPolicies())
		alerts, err := engine.AnalyzeEvents(ctx, events)
		if err != nil {
			return err
		}
		alerts = filterAlertCooldown(alerts, lastAlertAt, *alertCooldown, time.Now())
		if len(alerts) > 0 {
			if err := report.WriteText(os.Stdout, alerts); err != nil {
				cleanupFrame(imagePath, *keepFrames)
				return err
			}
			if *notify {
				notifyAlerts(alerts)
			}
		}
		if *verbose {
			fmt.Fprintf(os.Stderr, "opswatch: frame analyzed alerts=%d capture=%s resize=%s hash=%s vision=%s total=%s\n", len(alerts), captureDuration.Round(time.Millisecond), resizeDuration.Round(time.Millisecond), hashDuration.Round(time.Millisecond), visionDuration.Round(time.Millisecond), time.Since(frameStarted).Round(time.Millisecond))
		}
		cleanupFrame(imagePath, *keepFrames)

		if *once {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*interval):
		}
	}
}

func notifyAlerts(alerts []domain.Alert) {
	for _, alert := range alerts {
		if runtime.GOOS != "darwin" {
			continue
		}
		message := alert.Explanation
		if len(alert.Evidence) > 0 {
			message = alert.Evidence[0]
		}
		_ = exec.Command(
			"osascript",
			"-e",
			fmt.Sprintf("display notification %q with title %q subtitle %q", truncateForNotification(message, 120), "OpsWatch", strings.ToUpper(string(alert.Severity))+": "+alert.Title),
		).Run()
	}
}

func truncateForNotification(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	if maxLength <= 3 {
		return value[:maxLength]
	}
	return value[:maxLength-3] + "..."
}

func parseCaptureRect(value string) (capture.Rect, bool, error) {
	if strings.TrimSpace(value) == "" {
		return capture.Rect{}, false, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return capture.Rect{}, false, fmt.Errorf("--capture-rect must be x,y,width,height")
	}
	values := make([]int, 4)
	for i, part := range parts {
		parsed, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return capture.Rect{}, false, fmt.Errorf("--capture-rect contains invalid integer %q", part)
		}
		values[i] = parsed
	}
	rect := capture.Rect{X: values[0], Y: values[1], Width: values[2], Height: values[3]}
	if rect.Width <= 0 || rect.Height <= 0 {
		return capture.Rect{}, false, fmt.Errorf("--capture-rect width and height must be greater than zero")
	}
	return rect, true, nil
}

func cleanupFrame(path string, keep bool) {
	if keep {
		return
	}
	_ = os.Remove(path)
}

func filterAlertCooldown(alerts []domain.Alert, lastAlertAt map[string]time.Time, cooldown time.Duration, now time.Time) []domain.Alert {
	if cooldown <= 0 {
		return alerts
	}
	filtered := make([]domain.Alert, 0, len(alerts))
	for _, alert := range alerts {
		signature := alertSignature(alert)
		if lastSeen, ok := lastAlertAt[signature]; ok && now.Sub(lastSeen) < cooldown {
			continue
		}
		lastAlertAt[signature] = now
		filtered = append(filtered, alert)
	}
	return filtered
}

func alertSignature(alert domain.Alert) string {
	return string(alert.Severity) + "|" + alert.Title + "|" + strings.Join(alert.Evidence, "|")
}

type visionOptions struct {
	Provider         string
	Model            string
	OllamaEndpoint   string
	Timeout          time.Duration
	OllamaNumPredict int
}

func imageEvents(ctx context.Context, imagePath string, frame vision.FrameContext, options visionOptions) ([]domain.Event, error) {
	events := make([]domain.Event, 0, 3+len(frame.ProtectedDomains))
	now := time.Now().UTC()
	for _, domainName := range frame.ProtectedDomains {
		events = append(events, domain.Event{
			Timestamp: now,
			Source:    domain.SourceAPI,
			Text:      "Loaded protected domain policy",
			Context: map[string]string{
				"kind":   "protected_domain",
				"domain": domainName,
			},
		})
	}
	if frame.ExpectedAction != "" {
		events = append(events, domain.Event{
			Timestamp: now,
			Source:    domain.SourceRunbook,
			Text:      "Expected action",
			Context: map[string]string{
				"expected_action": frame.ExpectedAction,
			},
		})
	}
	if frame.Intent != "" {
		events = append(events, domain.Event{
			Timestamp: now,
			Source:    domain.SourceSpeech,
			Actor:     "operator",
			Text:      frame.Intent,
		})
	}

	client, err := newVisionClient(options)
	if err != nil {
		return nil, err
	}
	screenEvent, err := client.AnalyzeImage(ctx, imagePath, frame)
	if err != nil {
		return nil, err
	}
	events = append(events, screenEvent)
	return events, nil
}

func newVisionClient(options visionOptions) (vision.ImageAnalyzer, error) {
	switch strings.ToLower(strings.TrimSpace(options.Provider)) {
	case "", "openai":
		return vision.NewOpenAIClientFromEnv(options.Model)
	case "ollama":
		client := vision.NewOllamaClient(options.Model, options.OllamaEndpoint, options.Timeout)
		if options.OllamaNumPredict > 0 {
			client.Options = map[string]any{"num_predict": options.OllamaNumPredict}
		}
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported vision provider %q", options.Provider)
	}
}

func resizedImagePath(ctx context.Context, imagePath string, maxDimension int) (string, error) {
	ext := filepath.Ext(imagePath)
	if ext == "" {
		ext = ".png"
	}
	resizedPath := filepath.Join(os.TempDir(), fmt.Sprintf("opswatch-image-%d%s", time.Now().UnixNano(), ext))
	input, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(resizedPath, input, 0o600); err != nil {
		return "", err
	}
	if err := (capture.MacOSCapture{}).ResizeMaxDimension(ctx, resizedPath, maxDimension); err != nil {
		return "", err
	}
	return resizedPath, nil
}

func waitForNextFrame(ctx context.Context, interval time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(interval):
	}
}

func writeAlerts(format string, alerts []domain.Alert) error {
	switch format {
	case "text":
		return report.WriteText(os.Stdout, alerts)
	case "json":
		return report.WriteJSON(os.Stdout, alerts)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func splitCSV(value string) []string {
	var values []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, strings.ToLower(part))
		}
	}
	return values
}
