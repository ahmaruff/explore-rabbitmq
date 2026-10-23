package files

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

// sniffBytes is how much of the file is read to detect its content type.
// net/http only needs the first 512 bytes.
const sniffBytes = 512

const (
	defaultCacheControl = "public, max-age=31536000, immutable"
	privateCacheControl = "private, no-store"
)

// Upload validates and stores one file. The policy is passed by the caller so
// each endpoint states its own rule on the line where the upload happens.
func (s *Service) Upload(ctx context.Context, policy Policy, in UploadInput) (*File, error) {
	if err := checkPolicy(policy); err != nil {
		return nil, err
	}

	if in.Reader == nil {
		return nil, fmt.Errorf("files upload: reader is required")
	}

	if in.Size > 0 {
		if err := checkSize(policy, in.Size); err != nil {
			return nil, err
		}
	}

	originalName := cleanOriginalName(in.OriginalName)

	reader := bufio.NewReaderSize(in.Reader, sniffBytes)

	head, err := reader.Peek(sniffBytes)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read upload header: %w", err)
	}

	contentType, extension, err := detect(policy, head, originalName)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	key, err := buildKey(in.Folder, originalName, extension, now)
	if err != nil {
		return nil, err
	}

	// Read at most one byte over the limit so an overflow is detectable without
	// knowing the final size up front.
	counted := &countingReader{
		reader: io.LimitReader(reader, policy.MaxSizeBytes+1),
	}

	putErr := s.store.Put(ctx, key, counted, storage.PutOptions{
		ContentType:  contentType,
		CacheControl: defaultCacheControl,
	})
	if putErr != nil {
		s.discard(ctx, key)
		return nil, putErr
	}

	if counted.count > policy.MaxSizeBytes {
		s.discard(ctx, key)
		return nil, fmt.Errorf("%w: max %d bytes", ErrTooLarge, policy.MaxSizeBytes)
	}

	file := &File{
		Key:          key,
		OriginalName: originalName,
		ContentType:  contentType,
		Extension:    strings.TrimPrefix(extension, "."),
		Size:         counted.count,
		CreatedAt:    now,
	}

	return file, nil
}

// Open returns a seekable reader so the HTTP layer can hand it to
// http.ServeContent and support Range and conditional requests.
func (s *Service) Open(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	return s.store.Get(ctx, key)
}

func (s *Service) Delete(ctx context.Context, key string) error {
	return s.store.Delete(ctx, key)
}

func (s *Service) DeleteFolder(ctx context.Context, folder string) error {
	return s.store.DeletePrefix(ctx, folder)
}

// buildKey produces "{folder}/{yyyy}/{mm}/{prefix}_{id}{ext}".
func buildKey(folder string, originalName string, extension string, now time.Time) (string, error) {
	folder, err := cleanFolder(folder)
	if err != nil {
		return "", err
	}

	id, err := newKeyID()
	if err != nil {
		return "", err
	}

	name := id
	if prefix := namePrefix(originalName); prefix != "" {
		name = prefix + "_" + id
	}

	key := path.Join(folder, now.Format("2006"), now.Format("01"), name+extension)

	return key, nil
}

// maxNamePrefixLength caps the input as well as the output
const maxNamePrefixLength = 40

// namePrefix turns a client filename into a slug
func namePrefix(originalName string) string {
	base := strings.TrimSuffix(originalName, path.Ext(originalName))

	if len(base) > maxNamePrefixLength {
		base = base[:maxNamePrefixLength]
	}

	var builder strings.Builder
	builder.Grow(len(base))

	previousUnderscore := true // drops a leading separator

	for index := 0; index < len(base); index++ {
		char := base[index]

		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}

		isLetter := char >= 'a' && char <= 'z'
		isDigit := char >= '0' && char <= '9'

		if isLetter || isDigit {
			builder.WriteByte(char)
			previousUnderscore = false
			continue
		}

		if !previousUnderscore {
			builder.WriteByte('_')
			previousUnderscore = true
		}
	}

	return strings.TrimRight(builder.String(), "_")
}

// newKeyID returns 128 random bits as hex,
func newKeyID() (string, error) {
	buffer := make([]byte, 16)

	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate file key: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}

func cleanFolder(folder string) (string, error) {
	folder = strings.ReplaceAll(folder, "\\", "/")
	folder = strings.Trim(folder, "/")

	if folder == "" {
		return "", nil
	}

	for _, segment := range strings.Split(folder, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: %q", ErrInvalidFolder, folder)
		}
	}

	return folder, nil
}

func cleanOriginalName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.TrimSpace(name)

	if name == "." || name == "/" {
		return ""
	}

	return name
}

func (s *Service) discard(ctx context.Context, key string) {
	_ = s.store.Delete(ctx, key)
}

// countingReader counts what was actually read,
// so the stored size is the authoritative one
// rather than the size the client claimed.
type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count += int64(n)

	return n, err
}
