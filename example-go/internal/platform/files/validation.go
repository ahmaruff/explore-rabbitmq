package files

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
)

type AllowedType struct {
	ContentType string
	Extension   string // Includes the dot, e.g. ".png".
}

// Use Allow for a whitelist, or set AllowAny for "everything except Deny". The
// two cannot be combined, so a policy always says out loud which one it is.
type Policy struct {
	MaxSizeBytes int64
	Allow        []AllowedType

	// AllowAny accepts every detected content type that Deny does not block.
	AllowAny bool

	// Deny always wins, also in whitelist mode. Content types only.
	Deny []string
}

// policy helper
func ImagePolicy(maxSizeBytes int64) Policy {
	return Policy{
		MaxSizeBytes: maxSizeBytes,
		Allow: []AllowedType{
			{ContentType: "image/jpeg", Extension: ".jpg"},
			{ContentType: "image/png", Extension: ".png"},
			{ContentType: "image/gif", Extension: ".gif"},
			{ContentType: "image/webp", Extension: ".webp"},
		},
	}
}

// PDFPolicy accepts a single PDF.
func PDFPolicy(maxSizeBytes int64) Policy {
	return Policy{
		MaxSizeBytes: maxSizeBytes,
		Allow: []AllowedType{
			{ContentType: "application/pdf", Extension: ".pdf"},
		},
	}
}

func checkPolicy(policy Policy) error {
	if policy.MaxSizeBytes <= 0 {
		return fmt.Errorf("files policy: max size must be greater than zero")
	}

	if policy.AllowAny && len(policy.Allow) > 0 {
		return fmt.Errorf("files policy: Allow and AllowAny cannot be combined")
	}

	if !policy.AllowAny && len(policy.Allow) == 0 {
		return fmt.Errorf("files policy: give at least one allowed type, or set AllowAny")
	}

	for _, allowed := range policy.Allow {
		if allowed.ContentType == "" {
			return fmt.Errorf("files policy: allowed type has an empty content type")
		}

		extension := strings.TrimPrefix(allowed.Extension, ".")
		if extension == allowed.Extension || extension == "" {
			return fmt.Errorf("files policy: extension %q must start with a dot", allowed.Extension)
		}

		if !isAlphaNumeric(extension) {
			return fmt.Errorf("files policy: extension %q must be letters and digits", allowed.Extension)
		}
	}

	for _, denied := range policy.Deny {
		if strings.TrimSpace(denied) == "" {
			return fmt.Errorf("files policy: denied type is empty")
		}
	}

	return nil
}

func checkSize(policy Policy, size int64) error {
	if size == 0 {
		return ErrEmptyFile
	}

	if size > policy.MaxSizeBytes {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrTooLarge, size, policy.MaxSizeBytes)
	}

	return nil
}

// detect returns the content type and the extension to store the file under.
// The bytes decide the type; the original name is only consulted when AllowAny
// is set, because then no list entry provides an extension.
func detect(policy Policy, head []byte, originalName string) (string, string, error) {
	if len(head) == 0 {
		return "", "", ErrEmptyFile
	}

	contentType := baseType(http.DetectContentType(head))

	for _, denied := range policy.Deny {
		if baseType(denied) == contentType {
			return "", "", fmt.Errorf("%w: %s", ErrUnsupportedType, contentType)
		}
	}

	for _, allowed := range policy.Allow {
		if baseType(allowed.ContentType) == contentType {
			return contentType, allowed.Extension, nil
		}
	}

	if policy.AllowAny {
		return contentType, safeExtension(originalName), nil
	}

	return "", "", fmt.Errorf("%w: %s", ErrUnsupportedType, contentType)
}

// baseType drops the parameters DetectContentType appends, so
// "text/html; charset=utf-8" matches a Deny entry of "text/html".
func baseType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(contentType))
	}

	return strings.ToLower(mediaType)
}

// safeExtension keeps an extension from the client name only when it is short
// and simple; anything else falls back to ".bin".
func safeExtension(name string) string {
	extension := path.Ext(name)

	if len(extension) < 2 || len(extension) > 11 {
		return ".bin"
	}

	if !isAlphaNumeric(extension[1:]) {
		return ".bin"
	}

	return strings.ToLower(extension)
}

func isAlphaNumeric(text string) bool {
	for _, char := range text {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := char >= '0' && char <= '9'

		if !isLetter && !isDigit {
			return false
		}
	}

	return true
}
