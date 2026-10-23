package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"gocloud.dev/blob/fileblob"
)

const localRoot = "storage/app"

type localStorage struct {
	*blobStorage
}

var _ Store = (*localStorage)(nil)

func newLocal(root string) (*localStorage, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}

	if err := os.MkdirAll(absoluteRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}

	bucket, err := fileblob.OpenBucket(absoluteRoot, &fileblob.Options{
		NoTempDir: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open local storage: %w", err)
	}

	return &localStorage{
		blobStorage: &blobStorage{bucket: bucket},
	}, nil
}
