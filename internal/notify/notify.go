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
	Notify(ctx context.Context, title, subtitle, message, link string) error
}

type notifier struct {
	logger *slog.Logger
}

func NewNotifier(logger *slog.Logger) Notifier {
	return &notifier{logger: logger}
}

func (n *notifier) Notify(ctx context.Context, title, subtitle, message, link string) error {
	title = truncateForNotification(title, 128)
	subtitle = truncateForNotification(subtitle, 256)
	message = truncateForNotification(message, 512)

	dialogText := message
	if subtitle != "" {
		if dialogText != "" {
			dialogText = fmt.Sprintf("%s\n\n%s", subtitle, message)
		} else {
			dialogText = subtitle
		}
	}
	if dialogText == "" {
		dialogText = title
	}

	buttons := `{"Dismiss"}`
	defaultButton := `"Dismiss"`
	openButton := ""
	if strings.TrimSpace(link) != "" {
		buttons = `{"Open PR","Dismiss"}`
		defaultButton = `"Open PR"`
		openButton = "Open PR"
	}

	script := fmt.Sprintf(`set dialogResult to display dialog %q with title %q buttons %s default button %s giving up after 20`, dialogText, title, buttons, defaultButton)
	if openButton != "" {
		script += fmt.Sprintf(`\nif gave up of dialogResult is false and button returned of dialogResult is %q then
	do shell script "open " & quoted form of %q
end if`, openButton, link)
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
