package files

import (
	"context"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/config"
)

func TestPublicBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://s3:9000":            "http://localhost:9000",
		"http://minio:9000":         "http://localhost:9000",
		"https://s3.example.org":    "https://s3.example.org",
		"http://seaweed.local:8333": "http://seaweed.local:8333",
	} {
		u, err := publicBaseURL(&config.Config{S3Endpoint: in})
		if err != nil || u.String() != want {
			t.Errorf("publicBaseURL(%s) = %v %v, want %s", in, u, err, want)
		}
	}
	t.Setenv("S3_PUBLIC_BASE_URL", "https://files.example.org")
	if u, _ := publicBaseURL(&config.Config{S3Endpoint: "http://s3:9000"}); u.String() != "https://files.example.org" {
		t.Errorf("S3_PUBLIC_BASE_URL must win, got %s", u)
	}
}

func TestRewriteURL(t *testing.T) {
	base, _ := url.Parse("https://files.example.org")
	c := &Client{publicBaseURL: base}
	in, _ := url.Parse("http://s3:9000/bucket/key?X-Amz-Signature=abc")
	out := c.rewriteURL(in)
	if out.String() != "https://files.example.org/bucket/key?X-Amz-Signature=abc" {
		t.Fatalf("rewrite: %s", out)
	}
	if (&Client{}).rewriteURL(in) != in {
		t.Fatal("without a public base the URL stays as it is")
	}
}

// TestClientAgainstS3 round-trips an object through a real S3 server (the
// dev stack's SeaweedFS) when STASH_TEST_S3_ENDPOINT is set, e.g.
// http://localhost:9000 with STASH_TEST_S3_ACCESS_KEY / _SECRET_KEY.
func TestClientAgainstS3(t *testing.T) {
	endpoint := os.Getenv("STASH_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("STASH_TEST_S3_ENDPOINT not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := New(ctx, &config.Config{
		S3Endpoint: endpoint, S3Bucket: "stash-test", S3Region: "us-east-1", S3UsePathStyle: true,
		S3AccessKeyID: os.Getenv("STASH_TEST_S3_ACCESS_KEY"), S3SecretAccessKey: os.Getenv("STASH_TEST_S3_SECRET_KEY"),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	key := newBlobObjectKey("item", uuid.New(), ".txt")
	if err := c.PutObject(ctx, key, strings.NewReader("hello"), 5, "text/plain"); err != nil {
		t.Fatalf("put: %v", err)
	}
	rc, err := c.GetObject(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(body) != "hello" {
		t.Fatalf("body %q", body)
	}
	if u, err := c.PresignedGet(ctx, key, time.Minute); err != nil || !strings.Contains(u, "X-Amz-Signature") {
		t.Fatalf("presign: %s %v", u, err)
	}
	if err := c.Remove(ctx, key); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := c.GetObject(ctx, key); err == nil {
		t.Fatal("removed object still readable")
	}
}
