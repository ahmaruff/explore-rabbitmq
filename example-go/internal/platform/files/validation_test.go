package files

import (
	"errors"
	"testing"
)

func TestCheckPolicyRejectsUnusablePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
	}{
		{
			name:   "zero max size",
			policy: Policy{AllowAny: true},
		},
		{
			name:   "no allow and no allow any",
			policy: Policy{MaxSizeBytes: 1024},
		},
		{
			name: "allow combined with allow any",
			policy: Policy{
				MaxSizeBytes: 1024,
				AllowAny:     true,
				Allow:        []AllowedType{{ContentType: "image/png", Extension: ".png"}},
			},
		},
		{
			name: "extension without dot",
			policy: Policy{
				MaxSizeBytes: 1024,
				Allow:        []AllowedType{{ContentType: "image/png", Extension: "png"}},
			},
		},
		{
			name: "extension with slash",
			policy: Policy{
				MaxSizeBytes: 1024,
				Allow:        []AllowedType{{ContentType: "image/png", Extension: ".p/ng"}},
			},
		},
		{
			name: "empty denied type",
			policy: Policy{
				MaxSizeBytes: 1024,
				AllowAny:     true,
				Deny:         []string{" "},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := checkPolicy(test.policy); err == nil {
				t.Error("checkPolicy error = nil, want error")
			}
		})
	}
}

func TestCheckPolicyAcceptsValidPolicies(t *testing.T) {
	valid := []Policy{
		ImagePolicy(1024),
		PDFPolicy(1024),
		{MaxSizeBytes: 1024, AllowAny: true},
		{MaxSizeBytes: 1024, AllowAny: true, Deny: []string{"text/html"}},
	}

	for _, policy := range valid {
		if err := checkPolicy(policy); err != nil {
			t.Errorf("checkPolicy error = %v, want nil", err)
		}
	}
}

func TestDetectWhitelist(t *testing.T) {
	policy := ImagePolicy(1024)

	valid := []struct {
		name        string
		head        []byte
		contentType string
		extension   string
	}{
		{name: "png", head: []byte("\x89PNG\r\n\x1a\n"), contentType: "image/png", extension: ".png"},
		{name: "jpeg", head: []byte("\xff\xd8\xff\xe0\x00\x10JFIF"), contentType: "image/jpeg", extension: ".jpg"},
		{name: "gif", head: []byte("GIF89a...."), contentType: "image/gif", extension: ".gif"},
	}

	for _, test := range valid {
		t.Run(test.name, func(t *testing.T) {
			contentType, extension, err := detect(policy, test.head, "ignored.dat")
			if err != nil {
				t.Fatalf("detect error = %v", err)
			}

			if contentType != test.contentType {
				t.Errorf("content type = %q, want %q", contentType, test.contentType)
			}

			if extension != test.extension {
				t.Errorf("extension = %q, want %q", extension, test.extension)
			}
		})
	}

	t.Run("pdf is not allowed", func(t *testing.T) {
		pdf := []byte("%PDF-1.7\n")

		if _, _, err := detect(policy, pdf, "doc.pdf"); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("detect error = %v, want ErrUnsupportedType", err)
		}
	})

	t.Run("empty is rejected", func(t *testing.T) {
		if _, _, err := detect(policy, nil, ""); !errors.Is(err, ErrEmptyFile) {
			t.Errorf("detect error = %v, want ErrEmptyFile", err)
		}
	})
}

func TestDetectAllowAny(t *testing.T) {
	policy := Policy{MaxSizeBytes: 1024, AllowAny: true}

	t.Run("text is accepted", func(t *testing.T) {
		contentType, extension, err := detect(policy, []byte("hello world"), "notes.txt")
		if err != nil {
			t.Fatalf("detect error = %v", err)
		}

		if contentType != "text/plain" {
			t.Errorf("content type = %q, want text/plain", contentType)
		}

		if extension != ".txt" {
			t.Errorf("extension = %q, want .txt", extension)
		}
	})

	t.Run("unknown bytes fall back to bin", func(t *testing.T) {
		contentType, extension, err := detect(policy, []byte{0x00, 0x01, 0x02, 0x03}, "blob")
		if err != nil {
			t.Fatalf("detect error = %v", err)
		}

		if contentType != "application/octet-stream" {
			t.Errorf("content type = %q, want application/octet-stream", contentType)
		}

		if extension != ".bin" {
			t.Errorf("extension = %q, want .bin", extension)
		}
	})

	t.Run("client extension is normalized", func(t *testing.T) {
		_, extension, err := detect(policy, []byte("hello world"), "photo.JPG")
		if err != nil {
			t.Fatalf("detect error = %v", err)
		}

		// With AllowAny the bytes decide the content type, but no list entry
		// supplies an extension, so the sanitized client extension is used.
		if extension != ".jpg" {
			t.Errorf("extension = %q, want .jpg", extension)
		}
	})
}

func TestDetectDenyWins(t *testing.T) {
	policy := Policy{
		MaxSizeBytes: 1024,
		AllowAny:     true,
		Deny:         []string{"text/html", "image/png"},
	}

	html := []byte("<!DOCTYPE html><html><body>hi</body></html>")
	if _, _, err := detect(policy, html, "x.html"); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("html detect error = %v, want ErrUnsupportedType", err)
	}

	png := []byte("\x89PNG\r\n\x1a\n")
	if _, _, err := detect(policy, png, "x.png"); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("png detect error = %v, want ErrUnsupportedType", err)
	}
}

func TestCheckSize(t *testing.T) {
	policy := ImagePolicy(10)

	if err := checkSize(policy, 0); !errors.Is(err, ErrEmptyFile) {
		t.Errorf("checkSize(0) error = %v, want ErrEmptyFile", err)
	}

	if err := checkSize(policy, 11); !errors.Is(err, ErrTooLarge) {
		t.Errorf("checkSize(11) error = %v, want ErrTooLarge", err)
	}

	if err := checkSize(policy, 10); err != nil {
		t.Errorf("checkSize(10) error = %v, want nil", err)
	}
}

func TestSafeExtension(t *testing.T) {
	tests := map[string]string{
		"notes.txt":           ".txt",
		"photo.PNG":           ".png",
		"archive.verylongext": ".bin",
		"no-extension":        ".bin",
		"":                    ".bin",
	}

	for name, want := range tests {
		if got := safeExtension(name); got != want {
			t.Errorf("safeExtension(%q) = %q, want %q", name, got, want)
		}
	}
}
