package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"
)

type blobStorage struct {
	bucket *blob.Bucket
}

func (s *blobStorage) Close() error {
	return s.bucket.Close()
}

func (s *blobStorage) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) error {
	key, err := normalizePath(key)
	if err != nil {
		return err
	}

	writerOptions := &blob.WriterOptions{
		ContentType:  opts.ContentType,
		CacheControl: opts.CacheControl,
	}

	w, err := s.bucket.NewWriter(ctx, key, writerOptions)
	if err != nil {
		return fmt.Errorf("create storage writer: %w", err)
	}

	_, copyErr := io.Copy(w, r)
	closeErr := w.Close()

	if copyErr != nil {
		return fmt.Errorf("write storage object: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("commit storage object: %w", closeErr)
	}

	return nil
}

func (s *blobStorage) Get(ctx context.Context, key string) (io.ReadSeekCloser, error) {
	key, err := normalizePath(key)
	if err != nil {
		return nil, err
	}

	r, err := s.bucket.NewReader(ctx, key, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("read storage object: %w", err)
	}

	// *blob.Reader is already an io.ReadSeekCloser.
	return r, nil
}

func (s *blobStorage) Exists(ctx context.Context, key string) (bool, error) {
	key, err := normalizePath(key)
	if err != nil {
		return false, err
	}

	_, err = s.bucket.Attributes(ctx, key)
	if err == nil {
		return true, nil
	}

	if isNotFound(err) {
		return false, nil
	}

	return false, fmt.Errorf("check storage object: %w", err)
}

func (s *blobStorage) Info(ctx context.Context, key string) (*ObjectInfo, error) {
	key, err := normalizePath(key)
	if err != nil {
		return nil, err
	}

	attrs, err := s.bucket.Attributes(ctx, key)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("get storage attributes: %w", err)
	}

	return &ObjectInfo{
		Key:          key,
		Size:         attrs.Size,
		ContentType:  attrs.ContentType,
		LastModified: attrs.ModTime,
	}, nil
}

func (s *blobStorage) Delete(ctx context.Context, key string) error {
	key, err := normalizePath(key)
	if err != nil {
		return err
	}

	if err := s.bucket.Delete(ctx, key); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return fmt.Errorf("delete storage object: %w", err)
	}

	return nil
}

func (s *blobStorage) DeletePrefix(ctx context.Context, prefix string) error {
	prefix, err := normalizePrefix(prefix)
	if err != nil {
		return err
	}

	if prefix == "" {
		return ErrInvalidPath
	}

	prefix += "/"

	iterator := s.bucket.List(&blob.ListOptions{Prefix: prefix})
	for {
		object, err := iterator.Next(ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("list storage objects: %w", err)
		}

		if strings.HasSuffix(object.Key, "/") {
			continue
		}

		if err := s.bucket.Delete(ctx, object.Key); err != nil && !isNotFound(err) {
			return fmt.Errorf("delete storage object %s: %w", object.Key, err)
		}
	}
}

func normalizePath(key string) (string, error) {
	if key == "" || strings.ContainsRune(key, '\x00') {
		return "", ErrInvalidPath
	}

	key = strings.ReplaceAll(key, "\\", "/")

	if strings.HasPrefix(key, "/") {
		return "", ErrInvalidPath
	}

	cleaned := path.Clean(key)

	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidPath
	}

	// A colon is a drive separator on Windows and never needed in a key.
	if strings.ContainsRune(cleaned, ':') {
		return "", ErrInvalidPath
	}

	return cleaned, nil
}

func normalizePrefix(prefix string) (string, error) {
	if strings.ContainsRune(prefix, '\x00') {
		return "", ErrInvalidPath
	}

	prefix = strings.ReplaceAll(prefix, "\\", "/")

	if strings.HasPrefix(prefix, "/") {
		return "", ErrInvalidPath
	}

	cleaned := path.Clean(prefix)

	if cleaned == "." {
		return "", nil
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidPath
	}

	return strings.TrimSuffix(cleaned, "/"), nil
}

func isNotFound(err error) bool {
	return gcerrors.Code(err) == gcerrors.NotFound
}
