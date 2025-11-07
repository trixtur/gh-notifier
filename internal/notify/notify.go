package notify

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"unicode/utf8"
)

type Notifier interface {
	Notify(ctx context.Context, title, subtitle, message string) error
}

type notifier struct {
	logger *slog.Logger
}

func NewNotifier(logger *slog.Logger) Notifier {
	return &notifier{logger: logger}
}

func (n *notifier) Notify(ctx context.Context, title, subtitle, message string) error {
	title = truncateForNotification(title, 128)
	subtitle = truncateForNotification(subtitle, 256)
	message = truncateForNotification(message, 512)

	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	if subtitle != "" {
		script = fmt.Sprintf(`display notification %q with title %q subtitle %q`, message, title, subtitle)
	}

	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		if n.logger != nil {
			n.logger.Warn("failed to send notification", slog.String("error", err.Error()))
		}
		return fmt.Errorf("osascript notification: %w", err)
	}
	return nil
}

func truncateForNotification(input string, max int) string {
	input = strings.TrimSpace(input)
	if max <= 0 {
		return input
	}
	if utf8.RuneCountInString(input) <= max {
		return input
	}
	runes := []rune(input)
	return strings.TrimSpace(string(runes[:max])) + "…"
}
