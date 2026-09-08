package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIO is the production Store backend: MinIO/S3, path-style, no TLS by
// default (matches deploy/presto/etc/catalog/lake.properties so the Go
// writer and Presto's Hive connector agree on how to reach the same
// objects). Requires Docker (see docs/DECISIONS.md) — exercised by
// integration tests only, never by the unit test suite.
type MinIO struct {
	client *minio.Client
	bucket string
}

// NewMinIO connects to a MinIO/S3-compatible endpoint. endpoint is host:port
// with no scheme (e.g. "localhost:9000").
func NewMinIO(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIO, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("store: connect to minio at %q: %w", endpoint, err)
	}
	return &MinIO{client: client, bucket: bucket}, nil
}

func (m *MinIO) Put(ctx context.Context, key string, data []byte) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("store: minio put %q: %w", key, err)
	}
	return nil
}

func (m *MinIO) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("store: minio get %q: %w", key, err)
	}
	defer obj.Close()

	b, err := io.ReadAll(obj)
	if err != nil {
		var errResp minio.ErrorResponse
		if errors.As(err, &errResp) && errResp.Code == "NoSuchKey" {
			return nil, fmt.Errorf("store: get %q: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("store: minio read %q: %w", key, err)
	}
	if len(b) == 0 {
		if _, statErr := m.client.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{}); statErr != nil {
			return nil, fmt.Errorf("store: get %q: %w", key, ErrNotFound)
		}
	}
	return b, nil
}

func (m *MinIO) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for obj := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("store: minio list %q: %w", prefix, obj.Err)
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}
