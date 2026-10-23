package files

import (
	"crypto/hmac"
	"errors"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

// Service implements http.Handler so the HTTP layer only has to mount it under
// RoutePrefix. This package does not import a router.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	// A link that carries a signature must have a valid one even when signatures
	// are optional, so a broken link fails loudly instead of silently working.
	signedRequest := s.requireSignature || r.URL.Query().Has("signature")

	if signedRequest {
		if !s.signatureValid(r, key) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	object, err := s.store.Get(r.Context(), key)
	if err != nil {
		writeFileError(w, r, err)
		return
	}

	defer object.Close()

	info, err := s.store.Info(r.Context(), key)
	if err != nil {
		writeFileError(w, r, err)
		return
	}

	contentType := info.ContentType
	if contentType != "" && contentType != "application/octet-stream" {
		w.Header().Set("Content-Type", contentType)
	}

	if signedRequest {
		w.Header().Set("Cache-Control", privateCacheControl)
	} else {
		w.Header().Set("Cache-Control", defaultCacheControl)
	}

	// Never let a browser second-guess the stored type, and sandbox documents
	// that could otherwise execute script in this origin.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")

	if isUnsafeInlineContentType(contentType) {
		w.Header().Set("Content-Disposition", "attachment")
	}

	// ServeContent adds Range, HEAD, If-Modified-Since and If-None-Match.
	http.ServeContent(w, r, path.Base(key), info.LastModified, object)
}

func isUnsafeInlineContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))

	switch mediaType {
	case "text/html", "application/xhtml+xml", "image/svg+xml", "text/xml", "application/xml":
		return true
	default:
		return false
	}
}

func (s *Service) signatureValid(r *http.Request, key string) bool {
	query := r.URL.Query()

	expiresParam := query.Get("expires")
	signature := query.Get("signature")

	if expiresParam == "" || signature == "" {
		return false
	}

	expires, err := strconv.ParseInt(expiresParam, 10, 64)
	if err != nil {
		return false
	}

	if time.Now().Unix() > expires {
		return false
	}

	expected := signatureFor(key, expires, s.signingKey)

	return hmac.Equal([]byte(signature), []byte(expected))
}

func writeFileError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		http.NotFound(w, r)

	case errors.Is(err, storage.ErrInvalidPath):
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

	default:
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
