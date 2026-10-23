package textutil

import (
	"strings"
	"testing"
)

func TestConvertLinksToAnchorWrapsPlainURL(t *testing.T) {
	got := ConvertLinksToAnchor("see https://example.com/page")

	want := `<a href="https://example.com/page" rel="noopener noreferrer nofollow">https://example.com/page</a>`

	if !strings.Contains(got, want) {
		t.Errorf("ConvertLinksToAnchor() = %q, want %q", got, want)
	}
}

func TestConvertLinksToAnchorAddsSchemeToWWW(t *testing.T) {
	got := ConvertLinksToAnchor("see www.example.com")

	if !strings.Contains(got, `href="https://www.example.com"`) {
		t.Errorf("ConvertLinksToAnchor() = %q, want https://www.example.com href", got)
	}
}

func TestConvertLinksToAnchorDoesNotTouchAttributes(t *testing.T) {
	input := `<img src="https://example.com/pic.png">`

	got := ConvertLinksToAnchor(input)

	if got != input {
		t.Errorf("ConvertLinksToAnchor() = %q, want unchanged %q", got, input)
	}
}

func TestConvertLinksToAnchorEscapesQueryString(t *testing.T) {
	got := ConvertLinksToAnchor("https://example.com/?a=1&b=2")

	if !strings.Contains(got, `href="https://example.com/?a=1&amp;b=2"`) {
		t.Errorf("ConvertLinksToAnchor() = %q, want escaped href", got)
	}
}
