package terminalscrape

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/vdplabs/opswatch/internal/domain"
	"github.com/vdplabs/opswatch/internal/vision"
)

var commandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bkubectl\s+[^\n]+`),
	regexp.MustCompile(`\bterraform\s+[^\n]+`),
	regexp.MustCompile(`\bhelm\s+[^\n]+`),
	regexp.MustCompile(`\baws\s+[^\n]+`),
	regexp.MustCompile(`\bgcloud\s+[^\n]+`),
	regexp.MustCompile(`\baz\s+[^\n]+`),
	regexp.MustCompile(`\bvault\s+[^\n]+`),
	regexp.MustCompile(`\bnomad\s+[^\n]+`),
	regexp.MustCompile(`\bconsul\s+[^\n]+`),
	regexp.MustCompile(`\broute53\b[^\n]+`),
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func SupportedApp(owner string) bool {
	switch normalizeOwner(owner) {
	case "terminal", "iterm", "iterm2":
		return true
	default:
		return false
	}
}

func ExtractEvent(ctx context.Context, frame vision.FrameContext) (domain.Event, bool, error) {
	owner := normalizeOwner(frame.WindowOwner)
	if !SupportedApp(owner) {
		return domain.Event{}, false, nil
	}

	var (
		content string
		err     error
	)
	switch owner {
	case "terminal":
		content, err = terminalContents(ctx, frame.WindowTitle)
	case "iterm", "iterm2":
		content, err = iTermContents(ctx, frame.WindowTitle)
	}
	if err != nil {
		return domain.Event{}, true, err
	}

	command := extractCommand(content)
	if command == "" {
		return domain.Event{}, true, fmt.Errorf("could not infer command from %s window", frame.WindowOwner)
	}

	return domain.Event{
		Timestamp: time.Now().UTC(),
		Source:    domain.SourceTerminal,
		Actor:     frame.Actor,
		Text:      command,
		Context: map[string]string{
			"app":         frame.WindowOwner,
			"command":     command,
			"environment": frame.Environment,
		},
	}, true, nil
}

func terminalContents(ctx context.Context, windowTitle string) (string, error) {
	script := `
on run argv
	tell application "Terminal"
		if (count of windows) = 0 then return ""
		repeat with w in windows
			try
				if name of w is equal to item 1 of argv then
					return contents of selected tab of w
				end if
			end try
		end repeat
		return contents of selected tab of front window
	end tell
end run`
	return runAppleScript(ctx, script, windowTitle)
}

func iTermContents(ctx context.Context, windowTitle string) (string, error) {
	script := `
on run argv
	tell application "iTerm2"
		if (count of windows) = 0 then return ""
		repeat with w in windows
			try
				if name of w is equal to item 1 of argv then
					return contents of current session of current tab of w
				end if
			end try
		end repeat
		return contents of current session of current window
	end tell
end run`
	return runAppleScript(ctx, script, windowTitle)
}

func runAppleScript(ctx context.Context, script, arg string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-s", "s", "-", arg)
	cmd.Stdin = strings.NewReader(script)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("osascript failed: %s", message)
	}
	return stdout.String(), nil
}

func extractCommand(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := normalizeLine(lines[i])
		if line == "" {
			continue
		}
		if command := matchCommand(line); command != "" {
			return command
		}
	}
	return ""
}

func matchCommand(line string) string {
	lower := strings.ToLower(line)
	for _, pattern := range commandPatterns {
		match := pattern.FindString(lower)
		if match == "" {
			continue
		}
		index := strings.Index(lower, match)
		if index < 0 {
			continue
		}
		return strings.TrimSpace(line[index:])
	}
	return ""
}

func normalizeLine(line string) string {
	line = ansiRegex.ReplaceAllString(line, "")
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "\r", "")
	return line
}

func normalizeOwner(owner string) string {
	owner = strings.ToLower(strings.TrimSpace(owner))
	owner = strings.ReplaceAll(owner, " ", "")
	return owner
}
