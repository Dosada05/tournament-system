package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings" // Добавлен для очистки ETag

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config" // Используем этот импорт для config
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type CloudflareR2UploaderConfig struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	PublicBaseURL   string
}

type cloudflareR2Uploader struct {
	s3Client      *s3.Client
	bucketName    string
	publicBaseURL string
}

func NewCloudflareR2Uploader(cfg CloudflareR2UploaderConfig) (FileUploader, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.BucketName == "" || cfg.PublicBaseURL == "" {
		return nil, errors.New("invalid Cloudflare R2 configuration: all fields are required")
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
		return aws.Endpoint{
			URL:           r2Endpoint, // URL твоего R2 эндпоинта
			SigningRegion: "auto",     // Говорит SDK использовать специальную логику подписи для R2
		}, nil
	})

	sdkCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver), // Наш кастомный резолвер для R2
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithRegion("auto"), // <--- ИСПРАВЛЕНИЕ: Явно указываем "auto" как регион
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS SDK config for R2: %w", err)
	}

	s3Client := s3.NewFromConfig(sdkCfg)

	return &cloudflareR2Uploader{
		s3Client:      s3Client,
		bucketName:    cfg.BucketName,
		publicBaseURL: cfg.PublicBaseURL,
	}, nil
}

func (u *cloudflareR2Uploader) Upload(ctx context.Context, key string, contentType string, reader io.Reader) (*UploadResult, error) {
	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucketName),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	result, err := u.s3Client.PutObject(ctx, putObjectInput)
	if err != nil {
		return nil, fmt.Errorf("failed to upload object to R2 (key: %s): %w", key, err)
	}

	location := u.GetPublicURL(key)
	etag := ""
	if result.ETag != nil {
		// ETag от S3-совместимых API часто приходит в двойных кавычках, их нужно убрать.
		etag = strings.Trim(*result.ETag, "\"")
	}

	return &UploadResult{
		Key:      key,
		Location: location,
		ETag:     etag,
	}, nil
}

func (u *cloudflareR2Uploader) Delete(ctx context.Context, key string) error {
	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucketName),
		Key:    aws.String(key),
	}

	_, err := u.s3Client.DeleteObject(ctx, deleteObjectInput)
	if err != nil {
		return fmt.Errorf("failed to delete object from R2 (key: %s): %w", key, err)
	}

	return nil
}

func (u *cloudflareR2Uploader) GetPublicURL(key string) string {
	if u.publicBaseURL == "" || key == "" {
		return "" // Не можем сформировать URL без этих данных
	}

	baseURL, err := url.Parse(u.publicBaseURL)
	if err != nil {
		// Логирование ошибки парсинга базового URL может быть полезно
		fmt.Printf("Error parsing publicBaseURL '%s': %v\n", u.publicBaseURL, err)
		return ""
	}

	finalPath := key
	if strings.HasSuffix(baseURL.Path, "/") && strings.HasPrefix(key, "/") {
		finalPath = strings.TrimPrefix(key, "/")
	} else if !strings.HasSuffix(baseURL.Path, "/") && !strings.HasPrefix(key, "/") && baseURL.Path != "" {
		// Если baseURL.Path не пустой и не заканчивается на /, а key не начинается на /
		// то нужно добавить / между ними, если только baseURL.Path это не просто хост.
		// Однако, обычно publicBaseURL уже содержит нужный слеш или не содержит путь вовсе.
		// ResolveReference должен справиться.
	}

	pathURL, err := url.Parse(finalPath)
	if err != nil {
		fmt.Printf("Error parsing key '%s' as URL path: %v\n", finalPath, err)
		return ""
	}

	fullURL := baseURL.ResolveReference(pathURL)
	return fullURL.String()
}
