package minio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	minioSDK "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrObjectNotFound indicates the requested object key does not exist in the MinIO bucket.
var ErrObjectNotFound = errors.New("minio: object not found")

// ObjectStat holds basic server-side metadata returned by MinIO StatObject.
type ObjectStat struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

// Storage abstracts MinIO object storage operations needed by the media module.
type Storage interface {
	EnsureBucket(ctx context.Context) error
	GeneratePresignedPut(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	StatObject(ctx context.Context, objectKey string) (ObjectStat, error)
	GetObjectBytes(ctx context.Context, objectKey string, maxReadBytes int64) ([]byte, error)
	RemoveObject(ctx context.Context, objectKey string) error
	ObjectURL(ctx context.Context, objectKey string) (string, error)
}

// Config configures the MinIO storage adapter.
type Config struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	Bucket        string
	UseSSL        bool
	PublicBaseURL string
	PresignGetTTL time.Duration
}

// Client wraps minio-go/v7 to implement Storage.
type Client struct {
	sdk           *minioSDK.Client
	endpoint      string
	bucket        string
	useSSL        bool
	publicBaseURL string
	presignGetTTL time.Duration
}

var _ Storage = (*Client)(nil)

// NewClient initializes a MinIO client for the configured endpoint and bucket.
func NewClient(cfg Config) (*Client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return nil, errors.New("minio endpoint is required")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, errors.New("minio bucket is required")
	}

	sdk, err := minioSDK.New(endpoint, &minioSDK.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		BucketLookup: minioSDK.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	presignGetTTL := cfg.PresignGetTTL
	if presignGetTTL <= 0 {
		presignGetTTL = 1 * time.Hour
	}

	return &Client{
		sdk:           sdk,
		endpoint:      endpoint,
		bucket:        bucket,
		useSSL:        cfg.UseSSL,
		publicBaseURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		presignGetTTL: presignGetTTL,
	}, nil
}

// EnsureBucket creates the target bucket if it does not already exist.
// It retries briefly if MinIO has just started and reports "Server not initialized yet".
func (c *Client) EnsureBucket(ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 25; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(150 * time.Millisecond):
			}
		}

		exists, err := c.sdk.BucketExists(ctx, c.bucket)
		if err != nil {
			lastErr = fmt.Errorf("check minio bucket %q: %w", c.bucket, err)
			if isTransientInitError(err) {
				continue
			}
			return lastErr
		}
		if exists {
			return nil
		}

		err = c.sdk.MakeBucket(ctx, c.bucket, minioSDK.MakeBucketOptions{})
		if err != nil {
			resp := minioSDK.ToErrorResponse(err)
			if resp.Code == "BucketAlreadyOwnedByYou" || resp.Code == "BucketAlreadyExists" {
				return nil
			}
			lastErr = fmt.Errorf("make minio bucket %q: %w", c.bucket, err)
			if isTransientInitError(err) {
				continue
			}
			return lastErr
		}
		return nil
	}
	return lastErr
}

func isTransientInitError(err error) bool {
	if err == nil {
		return false
	}
	resp := minioSDK.ToErrorResponse(err)
	if resp.Code == "ServerNotInitialized" || resp.StatusCode == http.StatusServiceUnavailable {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not initialized") || strings.Contains(msg, "please try again")
}

// GeneratePresignedPut generates a short-lived presigned PUT URL for uploading directly to MinIO.
func (c *Client) GeneratePresignedPut(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	u, err := c.sdk.PresignedPutObject(ctx, c.bucket, objectKey, ttl)
	if err != nil {
		return "", fmt.Errorf("presign put %q: %w", objectKey, err)
	}
	return u.String(), nil
}

// StatObject queries MinIO for object metadata without downloading the body.
func (c *Client) StatObject(ctx context.Context, objectKey string) (ObjectStat, error) {
	info, err := c.sdk.StatObject(ctx, c.bucket, objectKey, minioSDK.StatObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return ObjectStat{}, ErrObjectNotFound
		}
		return ObjectStat{}, fmt.Errorf("stat object %q: %w", objectKey, err)
	}
	return ObjectStat{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}, nil
}

// GetObjectBytes downloads up to maxReadBytes from MinIO.
// Callers inspecting an upload with limit N should pass maxReadBytes = N + 1 so oversized objects
// are detected deterministically without buffering unbounded memory.
func (c *Client) GetObjectBytes(ctx context.Context, objectKey string, maxReadBytes int64) ([]byte, error) {
	if _, err := c.StatObject(ctx, objectKey); err != nil {
		return nil, err
	}
	obj, err := c.sdk.GetObject(ctx, c.bucket, objectKey, minioSDK.GetObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("get object %q: %w", objectKey, err)
	}
	defer obj.Close()

	var reader io.Reader = obj
	if maxReadBytes > 0 {
		reader = io.LimitReader(obj, maxReadBytes)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("read object %q: %w", objectKey, err)
	}
	return data, nil
}

// RemoveObject deletes an object from the MinIO bucket. Missing objects are treated as already deleted (idempotent).
func (c *Client) RemoveObject(ctx context.Context, objectKey string) error {
	err := c.sdk.RemoveObject(ctx, c.bucket, objectKey, minioSDK.RemoveObjectOptions{})
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("remove object %q: %w", objectKey, err)
	}
	return nil
}

// ObjectURL returns the public URL for objectKey if PublicBaseURL is configured,
// or falls back to a presigned GET URL.
func (c *Client) ObjectURL(ctx context.Context, objectKey string) (string, error) {
	cleanKey := strings.TrimLeft(objectKey, "/")
	if c.publicBaseURL != "" {
		return fmt.Sprintf("%s/%s/%s", c.publicBaseURL, c.bucket, cleanKey), nil
	}
	u, err := c.sdk.PresignedGetObject(ctx, c.bucket, cleanKey, c.presignGetTTL, url.Values{})
	if err != nil {
		scheme := "http"
		if c.useSSL {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s/%s/%s", scheme, c.endpoint, c.bucket, cleanKey), nil
	}
	return u.String(), nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrObjectNotFound) {
		return true
	}
	resp := minioSDK.ToErrorResponse(err)
	if resp.StatusCode == http.StatusNotFound {
		return true
	}
	switch resp.Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchBucket":
		return true
	default:
		return false
	}
}
