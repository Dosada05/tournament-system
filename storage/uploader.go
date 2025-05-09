package storage

import (
	"context"
	"io"
)

type UploadResult struct {
	Key      string // Ключ (путь) загруженного файла в хранилище
	Location string // Полный URL файла (может быть пустым, если не предоставляется хранилищем напрямую)
	ETag     string // ETag объекта (полезно для кеширования)
}

type FileUploader interface {
	Upload(ctx context.Context, key string, contentType string, reader io.Reader) (*UploadResult, error)

	Delete(ctx context.Context, key string) error

	GetPublicURL(key string) string
}
