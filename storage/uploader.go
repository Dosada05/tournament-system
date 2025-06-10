package storage

import (
	"context"
	"io"
)

type UploadResult struct {
	Key      string
	Location string
	ETag     string
}

type FileUploader interface {
	Upload(ctx context.Context, key string, contentType string, reader io.Reader) (*UploadResult, error)

	Delete(ctx context.Context, key string) error

	GetPublicURL(key string) string
}
