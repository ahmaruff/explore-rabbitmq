package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n")

// memStore is a hand written storage.Store so these tests cover upload rules and
// key names without touching the filesystem. The real driver is covered in
// platform/storage.
type memStore struct {
	objects map[string][]byte
	options map[string]storage.PutOptions
}

func newMemStore() *memStore {
	return &memStore{
		objects: make(map[string][]byte),
		options: make(map[string]storage.PutOptions),
	}
}

func (s *memStore) Put(_ context.Context, key string, r io.Reader, opts storage.PutOptions) error {
	content, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	s.objects[key] = content
	s.options[key] = opts

	return nil
}

func (s *memStore) Get(_ context.Context, key string) (io.ReadSeekCloser, error) {
	if strings.HasPrefix(key, "../") || strings.Contains(key, "/../") {
		return nil, storage.ErrInvalidPath
	}

	content, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return memReader{bytes.NewReader(content)}, nil
}

func (s *memStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.objects[key]

	return ok, nil
}

func (s *memStore) Info(_ context.Context, key string) (*storage.ObjectInfo, error) {
	content, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return &storage.ObjectInfo{
		Key:         key,
		Size:        int64(len(content)),
		ContentType: s.options[key].ContentType,
	}, nil
}

func (s *memStore) Delete(_ context.Context, key string) error {
	if _, ok := s.objects[key]; !ok {
		return storage.ErrNotFound
	}

	delete(s.objects, key)
	delete(s.options, key)

	return nil
}

func (s *memStore) DeletePrefix(_ context.Context, prefix string) error {
	prefix = strings.Trim(prefix, "/") + "/"

	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			delete(s.objects, key)
			delete(s.options, key)
		}
	}

	return nil
}

func (s *memStore) Close() error {
	return nil
}

func (s *memStore) count() int {
	return len(s.objects)
}

type memReader struct {
	*bytes.Reader
}

func (memReader) Close() error {
	return nil
}

func newTestService(t *testing.T) (*Service, *memStore) {
	t.Helper()

	store := newMemStore()

	service, err := New(Config{Store: store})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	return service, store
}

func TestUpload(t *testing.T) {
	service, store := newTestService(t)

	file, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		Folder:       "communities/12/posts",
		OriginalName: `..\..\evil.png`,
		Size:         int64(len(pngHeader)),
		Reader:       bytes.NewReader(pngHeader),
	})
	if err != nil {
		t.Fatalf("Upload error = %v", err)
	}

	keyPattern := regexp.MustCompile(`^communities/12/posts/\d{4}/\d{2}/evil_[0-9a-f]{32}\.png$`)
	if !keyPattern.MatchString(file.Key) {
		t.Errorf("key = %q, want it to match %s", file.Key, keyPattern)
	}

	if file.OriginalName != "evil.png" {
		t.Errorf("original name = %q, want evil.png", file.OriginalName)
	}

	if file.ContentType != "image/png" {
		t.Errorf("content type = %q, want image/png", file.ContentType)
	}

	if file.Extension != "png" {
		t.Errorf("extension = %q, want png", file.Extension)
	}

	if file.Size != int64(len(pngHeader)) {
		t.Errorf("size = %d, want %d", file.Size, len(pngHeader))
	}

	if got := store.options[file.Key].ContentType; got != "image/png" {
		t.Errorf("stored content type = %q, want image/png", got)
	}

	reader, err := service.Open(context.Background(), file.Key)
	if err != nil {
		t.Fatalf("Open error = %v", err)
	}

	stored, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stored file error = %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close stored file error = %v", err)
	}

	if !bytes.Equal(stored, pngHeader) {
		t.Errorf("stored content = %v, want %v", stored, pngHeader)
	}

	if got := store.count(); got != 1 {
		t.Errorf("stored files = %d, want 1", got)
	}

	if err := service.Delete(context.Background(), file.Key); err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	if got := store.count(); got != 0 {
		t.Errorf("stored files after delete = %d, want 0", got)
	}
}

func TestUploadAnyFile(t *testing.T) {
	service, store := newTestService(t)

	file, err := service.Upload(context.Background(), Policy{MaxSizeBytes: 1024, AllowAny: true}, UploadInput{
		OriginalName: "notes.txt",
		Reader:       strings.NewReader("hello world"),
	})
	if err != nil {
		t.Fatalf("Upload error = %v", err)
	}

	if file.ContentType != "text/plain" {
		t.Errorf("content type = %q, want text/plain", file.ContentType)
	}

	if file.Extension != "txt" {
		t.Errorf("extension = %q, want txt", file.Extension)
	}

	if got := store.options[file.Key].ContentType; got != "text/plain" {
		t.Errorf("stored content type = %q, want text/plain", got)
	}
}

func TestUploadRejectsDeniedType(t *testing.T) {
	service, store := newTestService(t)

	policy := Policy{MaxSizeBytes: 1024, AllowAny: true, Deny: []string{"text/plain"}}

	_, err := service.Upload(context.Background(), policy, UploadInput{
		OriginalName: "notes.txt",
		Reader:       strings.NewReader("hello world"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("Upload error = %v, want ErrUnsupportedType", err)
	}

	if got := store.count(); got != 0 {
		t.Errorf("stored files = %d, want 0", got)
	}
}

func TestUploadRejectsInvalidPolicy(t *testing.T) {
	service, _ := newTestService(t)

	_, err := service.Upload(context.Background(), Policy{MaxSizeBytes: 1024}, UploadInput{
		Reader: bytes.NewReader(pngHeader),
	})
	if err == nil {
		t.Fatal("Upload error = nil, want policy error")
	}
}

func TestUploadRejectsKnownOversize(t *testing.T) {
	service, store := newTestService(t)

	_, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		Size:   2048,
		Reader: bytes.NewReader(pngHeader),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Upload error = %v, want ErrTooLarge", err)
	}

	if got := store.count(); got != 0 {
		t.Errorf("stored files = %d, want 0", got)
	}
}

func TestUploadRejectsStreamedOversize(t *testing.T) {
	service, store := newTestService(t)

	_, err := service.Upload(context.Background(), ImagePolicy(4), UploadInput{
		Reader: bytes.NewReader(pngHeader),
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Upload error = %v, want ErrTooLarge", err)
	}

	// The partial object must not survive the rejected upload.
	if got := store.count(); got != 0 {
		t.Errorf("stored files = %d, want 0", got)
	}
}

func TestUploadRejectsUnsupportedType(t *testing.T) {
	service, store := newTestService(t)

	_, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		OriginalName: "notes.txt",
		Reader:       strings.NewReader("hello world"),
	})
	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("Upload error = %v, want ErrUnsupportedType", err)
	}

	if got := store.count(); got != 0 {
		t.Errorf("stored files = %d, want 0", got)
	}
}

func TestUploadRejectsEmptyFile(t *testing.T) {
	service, _ := newTestService(t)

	_, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		Reader: bytes.NewReader(nil),
	})
	if !errors.Is(err, ErrEmptyFile) {
		t.Fatalf("Upload error = %v, want ErrEmptyFile", err)
	}
}

func TestUploadRejectsBadFolder(t *testing.T) {
	service, _ := newTestService(t)

	_, err := service.Upload(context.Background(), ImagePolicy(1024), UploadInput{
		Folder: "../../outside",
		Reader: bytes.NewReader(pngHeader),
	})
	if !errors.Is(err, ErrInvalidFolder) {
		t.Fatalf("Upload error = %v, want ErrInvalidFolder", err)
	}
}

func TestDeleteFolder(t *testing.T) {
	service, store := newTestService(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := service.Upload(ctx, ImagePolicy(1024), UploadInput{
			Folder: "communities/12/posts",
			Reader: bytes.NewReader(pngHeader),
		}); err != nil {
			t.Fatalf("Upload error = %v", err)
		}
	}

	if err := service.DeleteFolder(ctx, "communities/12/posts"); err != nil {
		t.Fatalf("DeleteFolder error = %v", err)
	}

	if got := store.count(); got != 0 {
		t.Errorf("stored files = %d, want 0", got)
	}
}

func TestBuildKeyKeepsReadablePrefix(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	key, err := buildKey("communities/12/posts", "My File (1).PDF", ".pdf", now)
	if err != nil {
		t.Fatalf("buildKey error = %v", err)
	}

	pattern := regexp.MustCompile(`^communities/12/posts/2026/10/my_file_1_[0-9a-f]{32}\.pdf$`)
	if !pattern.MatchString(key) {
		t.Errorf("key = %q, want it to match %s", key, pattern)
	}
}

func TestBuildKeyWithoutOriginalName(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	key, err := buildKey("", "", ".bin", now)
	if err != nil {
		t.Fatalf("buildKey error = %v", err)
	}

	pattern := regexp.MustCompile(`^2026/10/[0-9a-f]{32}\.bin$`)
	if !pattern.MatchString(key) {
		t.Errorf("key = %q, want it to match %s", key, pattern)
	}
}

func TestNamePrefix(t *testing.T) {
	tests := map[string]string{
		"My File (1).PDF":                "my_file_1",
		"evil.png":                       "evil",
		"file.tar.gz":                    "file_tar",
		"hello--world.txt":               "hello_world",
		"Ñoño.txt":                       "o_o", // non-ASCII bytes become separators
		"....":                           "",
		"":                               "",
		"___":                            "",
		strings.Repeat("a", 50) + ".png": strings.Repeat("a", 40),
	}

	for name, want := range tests {
		if got := namePrefix(name); got != want {
			t.Errorf("namePrefix(%q) = %q, want %q", name, got, want)
		}
	}
}
