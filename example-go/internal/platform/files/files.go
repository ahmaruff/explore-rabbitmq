package files

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

var (
	ErrTooLarge        = errors.New("file too large")
	ErrUnsupportedType = errors.New("unsupported file type")
	ErrEmptyFile       = errors.New("empty file")
	ErrInvalidFolder   = errors.New("invalid folder")
)

// File is the result of a successful upload. Persist Key and OriginalName: the
// original name only exists here and cannot be read back from storage.
type File struct {
	Key          string
	OriginalName string
	ContentType  string
	Extension    string
	Size         int64
	CreatedAt    time.Time
}

type UploadInput struct {
	Folder       string
	OriginalName string
	Size         int64
	Reader       io.Reader
}

// Config wires the service to a store. Upload rules are passed per call.
type Config struct {
	Store            storage.Store
	BaseURL          string
	SigningKey       string
	RequireSignature bool
}

type Service struct {
	store            storage.Store
	baseURL          string
	signingKey       []byte
	requireSignature bool
}

func New(cfg Config) (*Service, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("files: store is required")
	}

	if cfg.RequireSignature && cfg.SigningKey == "" {
		return nil, fmt.Errorf("files: signing key is required when a signature is required")
	}

	return &Service{
		store:            cfg.Store,
		baseURL:          strings.TrimRight(cfg.BaseURL, "/"),
		signingKey:       []byte(cfg.SigningKey),
		requireSignature: cfg.RequireSignature,
	}, nil
}
