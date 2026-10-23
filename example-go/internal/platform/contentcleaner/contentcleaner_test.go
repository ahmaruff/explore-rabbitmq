package contentcleaner

import (
	"strings"
	"testing"
)

func TestCleanTrimsText(t *testing.T) {
	got, err := New().Clean("  hello  ")

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if got != "hello" {
		t.Errorf("Clean = %q, want %q", got, "hello")
	}
}

func TestCleanLeavesEmptyInputEmpty(t *testing.T) {
	got, err := New().Clean("   ")

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if got != "" {
		t.Errorf("Clean = %q, want empty string", got)
	}
}

func TestCleanMasksEmails(t *testing.T) {
	got, err := New().Clean("contact me at ahmad@example.com")

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if strings.Contains(got, "ahmad@example.com") {
		t.Errorf("Clean = %q, want the email to be masked", got)
	}
}

func TestCleanRemovesJavascriptHref(t *testing.T) {
	got, err := New().Clean(`<a href="javascript:alert(1)">click</a>`)

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if strings.Contains(strings.ToLower(got), "javascript:") {
		t.Errorf("Clean = %q, want javascript URL removed", got)
	}
}

func TestCleanRemovesEventHandlers(t *testing.T) {
	got, err := New().Clean(`<img src=x onerror="alert(1)">`)

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if strings.Contains(strings.ToLower(got), "onerror") {
		t.Errorf("Clean = %q, want event handler removed", got)
	}
}

func TestCleanConvertsLinksToAnchor(t *testing.T) {
	got, err := New().Clean("see https://example.com/some-page")

	if err != nil {
		t.Fatalf("Clean returned error: %v", err)
	}

	if !strings.Contains(got, `<a href="https://example.com/some-page" rel="noopener noreferrer nofollow">https://example.com/some-page</a>`) {
		t.Errorf("Clean = %q, want the link wrapped in a safe anchor", got)
	}
}
