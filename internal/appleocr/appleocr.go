package appleocr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vdplabs/opswatch/internal/domain"
	"github.com/vdplabs/opswatch/internal/vision"
)

func HelperPath() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("OPSWATCH_OCR_HELPER")),
		"./macos/OpsWatchBar/.build/debug/OpsWatchOCR",
		"./macos/OpsWatchBar/.build/release/OpsWatchOCR",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if abs, err := filepath.Abs(candidate); err == nil {
			candidate = abs
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func Available() bool {
	return runtime.GOOS == "darwin" && HelperPath() != ""
}

func ExtractEvent(ctx context.Context, imagePath string, frame vision.FrameContext) (domain.Event, bool, error) {
	if runtime.GOOS != "darwin" {
		return domain.Event{}, false, nil
	}
	helper := HelperPath()
	if helper == "" {
		return domain.Event{}, false, nil
	}
	args := []string{
		"--image", imagePath,
		"--environment", frame.Environment,
		"--window-owner", frame.WindowOwner,
		"--window-title", frame.WindowTitle,
	}
	cmd := exec.CommandContext(ctx, helper, args...)
	output, err := cmd.Output()
	if err != nil {
		return domain.Event{}, true, fmt.Errorf("apple ocr helper failed: %w", err)
	}
	var event domain.Event
	if err := json.Unmarshal(output, &event); err != nil {
		return domain.Event{}, true, fmt.Errorf("apple ocr helper returned invalid json: %w", err)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = frameTimestamp()
	}
	if event.Actor == "" {
		event.Actor = frame.Actor
	}
	if event.Context == nil {
		event.Context = map[string]string{}
	}
	event.Context["ocr_provider"] = "apple_vision"
	if frame.Environment != "" && event.Context["environment"] == "" {
		event.Context["environment"] = frame.Environment
	}
	if !isMeaningful(event) {
		return domain.Event{}, false, nil
	}
	return event, true, nil
}

func isMeaningful(event domain.Event) bool {
	if event.Context["risk_hint"] == "high" {
		return true
	}
	if strings.TrimSpace(event.Context["command"]) != "" {
		return true
	}
	if strings.TrimSpace(event.Context["action"]) != "" && strings.TrimSpace(event.Context["resource_type"]) != "" {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(event.Text))
	return strings.Contains(text, "hosted zone") || strings.Contains(text, "kubectl") || strings.Contains(text, "terraform")
}
