package files

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

func uploadOnce(t *testing.T, service *Service, store *memStore) string {
	t.Helper()

	_, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		OriginalName: "pic.png",
		Reader:       bytes.NewReader(pngHeader),
	})
	if err != nil {
		t.Fatalf("Upload error = %v", err)
	}

	for key := range store.objects {
		return key
	}

	t.Fatal("upload stored no object")

	return ""
}

func TestServeHTTPGet(t *testing.T) {
	service, store := newTestService(t)
	key := uploadOnce(t, service, store)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/"+key, nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if body := recorder.Body.Bytes(); !bytes.Equal(body, pngHeader) {
		t.Errorf("body = %v, want %v", body, pngHeader)
	}

	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}

	if got := recorder.Header().Get("Cache-Control"); got != defaultCacheControl {
		t.Errorf("cache control = %q, want %q", got, defaultCacheControl)
	}

	if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("accept ranges = %q, want bytes", got)
	}

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("nosniff = %q, want nosniff", got)
	}
}

func TestServeHTTPHead(t *testing.T) {
	service, store := newTestService(t)
	key := uploadOnce(t, service, store)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodHead, "/"+key, nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if recorder.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", recorder.Body.Len())
	}

	if got := recorder.Header().Get("Content-Length"); got != "8" {
		t.Errorf("content length = %q, want 8", got)
	}
}

func TestServeHTTPRange(t *testing.T) {
	service, store := newTestService(t)
	key := uploadOnce(t, service, store)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/"+key, nil)
	request.Header.Set("Range", "bytes=0-3")

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", recorder.Code)
	}

	if body := recorder.Body.Bytes(); !bytes.Equal(body, pngHeader[:4]) {
		t.Errorf("body = %v, want %v", body, pngHeader[:4])
	}

	if got := recorder.Header().Get("Content-Range"); got != "bytes 0-3/8" {
		t.Errorf("content range = %q, want bytes 0-3/8", got)
	}
}

func TestServeHTTPNotFound(t *testing.T) {
	service, _ := newTestService(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/missing.png", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}

func TestServeHTTPInvalidPath(t *testing.T) {
	service, _ := newTestService(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/../secret", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	service, _ := newTestService(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/anything.png", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", recorder.Code)
	}

	if got := recorder.Header().Get("Allow"); got != "GET, HEAD" {
		t.Errorf("allow = %q, want GET, HEAD", got)
	}
}

func TestServeHTTPSniffsPlacedFile(t *testing.T) {
	service, store := newTestService(t)

	// A file dropped on disk by hand has no stored content type, so the handler
	// must let ServeContent derive it instead of sending octet-stream.
	const key = "2026/10/placed.png"
	store.objects[key] = pngHeader

	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/"+key, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q, want image/png", got)
	}
}

func TestServeHTTPRequireSignature(t *testing.T) {
	store := newMemStore()

	service, err := New(Config{
		Store:            store,
		SigningKey:       "test-signing-key",
		RequireSignature: true,
	})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	key := uploadOnce(t, service, store)

	// Mount it the same way platform/http does, so the prefix stripping is
	// exercised too.
	handler := http.StripPrefix(RoutePrefix, service)

	unsigned := httptest.NewRecorder()
	handler.ServeHTTP(unsigned, httptest.NewRequest(http.MethodGet, RoutePrefix+"/"+key, nil))

	if unsigned.Code != http.StatusForbidden {
		t.Errorf("unsigned status = %d, want 403", unsigned.Code)
	}

	signed, err := service.SignedURL(context.Background(), key, time.Minute)
	if err != nil {
		t.Fatalf("SignedURL error = %v", err)
	}

	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, signed, nil))

	if allowed.Code != http.StatusOK {
		t.Errorf("signed status = %d, want 200", allowed.Code)
	}

	if body := allowed.Body.Bytes(); !bytes.Equal(body, pngHeader) {
		t.Errorf("signed body = %v, want %v", body, pngHeader)
	}

	if got := allowed.Header().Get("Cache-Control"); got != privateCacheControl {
		t.Errorf("signed cache control = %q, want %q", got, privateCacheControl)
	}
}

func TestServeHTTPAttachesUnsafeInlineTypes(t *testing.T) {
	service, store := newTestService(t)

	const key = "2026/10/document.html"
	store.objects[key] = []byte("<script>alert(1)</script>")
	store.options[key] = storage.PutOptions{ContentType: "text/html; charset=utf-8"}

	recorder := httptest.NewRecorder()
	service.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/"+key, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Disposition"); got != "attachment" {
		t.Errorf("content disposition = %q, want attachment", got)
	}

	if got := recorder.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("content security policy = %q, want sandbox", got)
	}
}

func TestServeHTTPRejectsBadSignature(t *testing.T) {
	service, store := newTestService(t)
	key := uploadOnce(t, service, store)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/"+key+"?expires=9999999999&signature=deadbeef", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", recorder.Code)
	}
}
