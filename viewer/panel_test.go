package viewer

import (
	"strings"
	"testing"
)

func TestWrapText_ShortTextUnchanged(t *testing.T) {
	got := wrapText("hello world", 100)
	if got != "hello world" {
		t.Errorf("expected short text to pass through unchanged, got %q", got)
	}
}

func TestWrapText_ProseWrapsAtWordBoundaries(t *testing.T) {
	got := wrapText("the quick brown fox jumps over the lazy dog", 15)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 15 {
			t.Errorf("expected every line <= 15 chars, got %q (%d chars)", line, len(line))
		}
	}
	// Reassembling (with spaces collapsed back) should reproduce the words in order.
	if strings.Join(strings.Fields(got), " ") != "the quick brown fox jumps over the lazy dog" {
		t.Errorf("expected word order/content preserved, got %q", got)
	}
}

// TestWrapText_HardBreaksOverlongToken is the regression test for the
// original, most concrete complaint (an IAM policy JSON blob with no
// spaces rendering as one unbounded line): a single token longer than the
// target width must be hard-broken into width-sized chunks, not left
// unwrapped.
func TestWrapText_HardBreaksOverlongToken(t *testing.T) {
	longToken := strings.Repeat("x", 250) // simulates unbroken JSON
	got := wrapText(longToken, 40)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the long token to be broken across multiple lines, got %d line(s)", len(lines))
	}
	for _, line := range lines {
		if len(line) > 40 {
			t.Errorf("expected every line <= 40 chars, got %q (%d chars)", line, len(line))
		}
	}
	if strings.Join(lines, "") != longToken {
		t.Errorf("expected hard-wrapping to preserve every character, got %d chars back, want %d", len(strings.Join(lines, "")), len(longToken))
	}
}

func TestWrapText_ZeroWidthReturnsUnchanged(t *testing.T) {
	got := wrapText("hello world", 0)
	if got != "hello world" {
		t.Errorf("expected width<=0 to mean no wrapping, got %q", got)
	}
}

func TestWrapText_EmptyString(t *testing.T) {
	if got := wrapText("", 40); got != "" {
		t.Errorf("expected empty input to produce empty output, got %q", got)
	}
}
