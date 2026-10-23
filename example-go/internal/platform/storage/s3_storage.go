package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/s3blob"
)

// defaultSignedURLTTL is used when the caller does not ask for a specific ttl.
const defaultSignedURLTTL = 15 * time.Minute

type s3Storage struct {
	*blobStorage
}

var (
	_ Store  = (*s3Storage)(nil)
	_ Signer = (*s3Storage)(nil)
)

func newS3(ctx context.Context, bucketURL string) (*s3Storage, error) {
	if !strings.HasPrefix(bucketURL, "s3://") {
		return nil, fmt.Errorf("invalid S3 bucket URL")
	}

	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("open S3 storage: %w", err)
	}

	return &s3Storage{
		blobStorage: &blobStorage{bucket: bucket},
	}, nil
}

// SignedURL returns a presigned link. The bucket stays private; whoever holds
// the link can read the object until the ttl elapses.
func (s *s3Storage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	key, err := normalizePath(key)
	if err != nil {
		return "", err
	}

	if ttl <= 0 {
		ttl = defaultSignedURLTTL
	}

	signed, err := s.bucket.SignedURL(ctx, key, &blob.SignedURLOptions{Expiry: ttl})
	if err != nil {
		return "", fmt.Errorf("sign storage URL: %w", err)
	}

	return signed, nil
}
