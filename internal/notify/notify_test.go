package notify

import "testing"

func TestTruncateForNotification(t *testing.T) {
	input := "hello world"
	if got := truncateForNotification(input, 0); got != input {
		t.Fatalf("expected original string when max=0, got %q", got)
	}

	if got := truncateForNotification(input, 50); got != input {
		t.Fatalf("expected no truncation, got %q", got)
	}

	long := "abcdefghijklmnopqrstuvwxyz"
	if got := truncateForNotification(long, 5); got != "abcde…" {
		t.Fatalf("unexpected truncated result = %q", got)
	}
}
