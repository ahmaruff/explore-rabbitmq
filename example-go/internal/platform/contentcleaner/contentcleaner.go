// Package contentcleaner prepares user generated text before it is stored.
// It has no dependency on profanity: filtering is a separate service.
package contentcleaner

import (
	"errors"
	"fmt"
	"strings"

	"rabbitmq-example/internal/platform/textutil"

	"github.com/sym01/htmlsanitizer"
)

var ErrSanitizeFailed = errors.New("failed to sanitize html")

type Cleaner struct{}

func New() *Cleaner {
	return &Cleaner{}
}

func (Cleaner) Clean(raw string) (string, error) {
	text := strings.TrimSpace(raw)

	if text == "" {
		return "", nil
	}

	text, err := htmlsanitizer.SanitizeString(text)

	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSanitizeFailed, err)
	}

	text = textutil.ConvertLinksToAnchor(text)
	text = textutil.MaskEmailsInText(text)
	text = textutil.MaskPhoneNumbersInText(text)

	return text, nil
}
