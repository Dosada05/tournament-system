package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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
			URL:           r2Endpoint,
			SigningRegion: "auto",
		}, nil
	})

	sdkCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithRegion("auto"),
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
		return ""
	}

	baseURL, err := url.Parse(u.publicBaseURL)
	if err != nil {
		// В реальном приложении здесь лучше использовать логгер вместо fmt.Printf
		fmt.Printf("Error parsing publicBaseURL '%s': %v\n", u.publicBaseURL, err)
		return ""
	}

	pathURL, err := url.Parse(key) // Ключ может быть просто путем, url.Parse справится
	if err != nil {
		fmt.Printf("Error parsing key '%s' as URL path: %v\n", key, err)
		return ""
	}

	fullURL := baseURL.ResolveReference(pathURL)
	return fullURL.String()
}
