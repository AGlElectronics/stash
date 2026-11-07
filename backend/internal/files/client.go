// Package files wraps an S3-compatible object storage client (SeaweedFS in dev)
// and provides presigned URL helpers and a service that registers file
// metadata in PostgreSQL.
package files

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/AGlElectronics/stash/backend/internal/config"
)

// Client wraps a *minio.Client plus the configured bucket and a public-facing
// base URL used when rewriting host addresses for presigned URLs that the
// browser needs to reach (e.g. s3:9000 -> localhost:9000).
type Client struct {
	mc            *minio.Client
	bucket        string
	publicBaseURL *url.URL
}

// New constructs a Client and ensures the bucket exists.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	endpointURL, err := url.Parse(cfg.S3Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse S3_ENDPOINT: %w", err)
	}
	host := endpointURL.Host
	if host == "" {
		host = endpointURL.Path
	}
	useSSL := endpointURL.Scheme == "https"

	// Without explicit BucketLookup the SDK auto-detects, which can flip
	// between path- and virtual-hosted style depending on the host name.
	// Pin to path-style when configured (default for the compose S3 server) so
	// the signed canonical resource matches what the browser actually sends.
	bucketLookup := minio.BucketLookupAuto
	if cfg.S3UsePathStyle {
		bucketLookup = minio.BucketLookupPath
	}

	mc, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
		Secure:       useSSL,
		Region:       cfg.S3Region,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, cfg.S3Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.S3Bucket, minio.MakeBucketOptions{Region: cfg.S3Region}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	publicBase, _ := publicBaseURL(cfg)

	return &Client{
		mc:            mc,
		bucket:        cfg.S3Bucket,
		publicBaseURL: publicBase,
	}, nil
}

func publicBaseURL(cfg *config.Config) (*url.URL, error) {
	// Allow an explicit override via env. Otherwise, swap the compose host
	// "s3" (formerly "minio") for "localhost" so presigned URLs returned to a
	// dev browser actually work.
	if v := getenv("S3_PUBLIC_BASE_URL"); v != "" {
		return url.Parse(v)
	}
	u, err := url.Parse(cfg.S3Endpoint)
	if err != nil {
		return nil, err
	}
	if h := u.Hostname(); h == "s3" || h == "minio" {
		u.Host = strings.Replace(u.Host, h, "localhost", 1)
	}
	return u, nil
}

// PresignedPut returns a URL the browser can PUT bytes to.
func (c *Client) PresignedPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedPutObject(ctx, c.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return c.rewriteURL(u).String(), nil
}

// PresignedGet returns a URL the browser can GET bytes from.
func (c *Client) PresignedGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return c.rewriteURL(u).String(), nil
}

// PutObject streams bytes into the bucket. Used for backend-mediated uploads
// (avoids the CORS / browser-PUT dance during dev). Pass -1 as size if it is
// unknown; the SDK will buffer in memory up to part size.
func (c *Client) PutObject(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// GetObject opens a streaming reader for the object. Caller must Close.
func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	// Probe with Stat so missing keys surface as an error here instead of
	// later mid-stream where we cannot easily turn it into a 404.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return obj, nil
}

// Remove deletes an object. Used when a file metadata row is deleted.
func (c *Client) Remove(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// Bucket returns the configured bucket name.
func (c *Client) Bucket() string { return c.bucket }

func (c *Client) rewriteURL(u *url.URL) *url.URL {
	if c.publicBaseURL == nil {
		return u
	}
	out := *u
	out.Scheme = c.publicBaseURL.Scheme
	out.Host = c.publicBaseURL.Host
	return &out
}

// helper to read env without importing config inside files (avoids cycles).
func getenv(key string) string {
	return strings.TrimSpace(envLookup(key))
}
