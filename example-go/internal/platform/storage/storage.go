package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"rabbitmq-example/internal/platform/config"
)

var (
	ErrInvalidPath   = errors.New("invalid storage path")
	ErrNotFound      = errors.New("file not found")
	ErrUnknownDriver = errors.New("unknown storage driver")
)

type PutOptions struct {
	ContentType  string
	CacheControl string
}

type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
}

type Store interface {
	Put(ctx context.Context, key string, r io.Reader, opts PutOptions) error

	// Get returns a seekable reader so an HTTP handler can pass it to
	// http.ServeContent and get Range and conditional requests for free.+
	Get(ctx context.Context, key string) (io.ReadSeekCloser, error)

	Exists(ctx context.Context, key string) (bool, error)
	Info(ctx context.Context, key string) (*ObjectInfo, error)
	Delete(ctx context.Context, key string) error

	// DeletePrefix removes every object under a folder.
	DeletePrefix(ctx context.Context, prefix string) error

	Close() error
}

// Signer is implemented by drivers that can hand out a time limited link (S3).
// It is not part of Store on purpose: local storage has nothing to sign, so a
// caller type-asserts only when it actually wants a signed URL.
type Signer interface {
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

func New(ctx context.Context, cfg config.StorageConfig) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case config.StorageDriverLocal:
		return newLocal(localRoot)

	case config.StorageDriverS3:
		return newS3(ctx, cfg.S3BucketURL)

	case "":
		return nil, fmt.Errorf("%w: driver is empty", ErrUnknownDriver)

	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownDriver, cfg.Driver)
	}
}
