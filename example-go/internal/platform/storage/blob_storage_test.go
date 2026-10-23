package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) Store {
	t.Helper()

	store, err := newLocal(t.TempDir())
	if err != nil {
		t.Fatalf("newLocal error = %v", err)
	}

	t.Cleanup(func() {
		_ = store.Close()
	})

	return store
}

func TestNormalizePath(t *testing.T) {
	valid := []struct {
		name string
		key  string
		want string
	}{
		{name: "plain", key: "a/b.png", want: "a/b.png"},
		{name: "backslashes", key: `a\b.png`, want: "a/b.png"},
		{name: "redundant segments", key: "a//b/./c.png", want: "a/b/c.png"},
	}

	for _, test := range valid {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizePath(test.key)
			if err != nil {
				t.Fatalf("normalizePath(%q) error = %v", test.key, err)
			}

			if got != test.want {
				t.Errorf("normalizePath(%q) = %q, want %q", test.key, got, test.want)
			}
		})
	}

	invalid := []struct {
		name string
		key  string
	}{
		{name: "empty", key: ""},
		{name: "nul byte", key: "a\x00b"},
		{name: "leading slash", key: "/etc/passwd"},
		{name: "parent", key: "../secret"},
		{name: "nested parent", key: "a/../../secret"},
		{name: "current dir", key: "."},
		{name: "drive colon", key: "C:/windows"},
	}

	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizePath(test.key); !errors.Is(err, ErrInvalidPath) {
				t.Errorf("normalizePath(%q) error = %v, want ErrInvalidPath", test.key, err)
			}
		})
	}
}

func TestNormalizePrefix(t *testing.T) {
	got, err := normalizePrefix("")
	if err != nil || got != "" {
		t.Fatalf("normalizePrefix(\"\") = %q, %v; want \"\", nil", got, err)
	}

	got, err = normalizePrefix(`communities\12\`)
	if err != nil || got != "communities/12" {
		t.Fatalf("normalizePrefix = %q, %v; want communities/12, nil", got, err)
	}

	if _, err := normalizePrefix("../outside"); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("normalizePrefix(parent) error = %v, want ErrInvalidPath", err)
	}
}

func TestLocalStorageRoundTrip(t *testing.T) {
	ctx := context.Background()

	store := newTestStore(t)

	const key = "communities/12/posts/hello.txt"

	err := store.Put(ctx, key, strings.NewReader("hello"), PutOptions{
		ContentType:  "text/plain",
		CacheControl: "no-store",
	})
	if err != nil {
		t.Fatalf("Put error = %v", err)
	}

	reader, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read object error = %v", err)
	}

	// Get must be seekable so http.ServeContent can answer Range requests.
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek object error = %v", err)
	}

	reread := make([]byte, 2)
	if _, err := io.ReadFull(reader, reread); err != nil {
		t.Fatalf("read after seek error = %v", err)
	}

	if string(reread) != "he" {
		t.Errorf("read after seek = %q, want %q", reread, "he")
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close object error = %v", err)
	}

	if string(content) != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}

	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}

	if !exists {
		t.Error("Exists = false, want true")
	}

	info, err := store.Info(ctx, key)
	if err != nil {
		t.Fatalf("Info error = %v", err)
	}

	if info.Size != int64(len("hello")) {
		t.Errorf("Info size = %d, want %d", info.Size, len("hello"))
	}

	if err := store.DeletePrefix(ctx, "communities/12"); err != nil {
		t.Fatalf("DeletePrefix error = %v", err)
	}

	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists after delete error = %v", err)
	}

	if exists {
		t.Error("Exists after DeletePrefix = true, want false")
	}
}

func TestLocalStorageMissingObject(t *testing.T) {
	ctx := context.Background()

	store := newTestStore(t)

	exists, err := store.Exists(ctx, "nope.txt")
	if err != nil {
		t.Fatalf("Exists error = %v", err)
	}

	if exists {
		t.Error("Exists = true, want false")
	}

	if _, err := store.Get(ctx, "nope.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}

	if _, err := store.Info(ctx, "nope.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Info error = %v, want ErrNotFound", err)
	}

	if err := store.Delete(ctx, "nope.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestDeletePrefixRejectsEmpty(t *testing.T) {
	store := newTestStore(t)

	if err := store.DeletePrefix(context.Background(), ""); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("DeletePrefix(\"\") error = %v, want ErrInvalidPath", err)
	}
}
