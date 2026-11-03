package files

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

func TestNewBlobObjectKey_layout(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	k := newBlobObjectKey("item", id, ".png")
	wantPrefix := "stash/item/" + id.String() + "/"
	if !strings.HasPrefix(k, wantPrefix) || !strings.HasSuffix(k, ".png") {
		t.Fatalf("unexpected key %q (want prefix %q and .png suffix)", k, wantPrefix)
	}
	if strings.Count(k, "/") != 3 {
		t.Fatalf("expected 3 slashes in path, got %q", k)
	}
}

// Service.Upload validates inputs before touching object storage or the
// database, so we can exercise that path with a zero-valued service. The
// nil pool/client would explode if any branch slipped through, which makes
// these tests double-duty as guard rails.
func TestServiceUpload_Validation(t *testing.T) {
	t.Parallel()

	s := &Service{}
	id := uuid.New()
	body := bytes.NewBufferString("dummy")

	cases := []struct {
		name string
		in   UploadInput
		want string
	}{
		{
			name: "missing file_type",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         "",
				OriginalFilename: "a.png",
				Body:             body,
			},
			want: "invalid file_type",
		},
		{
			name: "invalid file_type",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         "bogus",
				OriginalFilename: "a.png",
				Body:             body,
			},
			want: "invalid file_type",
		},
		{
			name: "missing filename",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeImage,
				OriginalFilename: "",
				Body:             body,
			},
			want: "original_filename is required",
		},
		{
			name: "blank filename",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeImage,
				OriginalFilename: "   ",
				Body:             body,
			},
			want: "original_filename is required",
		},
		{
			name: "missing body",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeImage,
				OriginalFilename: "a.png",
				Body:             nil,
			},
			want: "file body is required",
		},
		{
			name: "image with non-image content type",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeImage,
				OriginalFilename: "a.png",
				ContentType:      "application/pdf",
				Body:             body,
			},
			want: "image/* content_type",
		},
		{
			name: "oversized image",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeImage,
				OriginalFilename: "huge.png",
				ContentType:      "image/png",
				Size:             MaxImageBytes + 1,
				Body:             body,
			},
			want: "file too large",
		},
		{
			name: "oversized attachment",
			in: UploadInput{
				EntityType:       "item",
				EntityID:         id,
				FileType:         domain.FileTypeAttachment,
				OriginalFilename: "huge.bin",
				ContentType:      "application/octet-stream",
				Size:             MaxAttachmentBytes + 1,
				Body:             body,
			},
			want: "file too large",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := s.Upload(context.Background(), tc.in)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			var aErr *httperror.Error
			if !errors.As(err, &aErr) {
				t.Fatalf("expected *httperror.Error, got %T: %v", err, err)
			}
			if aErr.Code != httperror.CodeValidation {
				t.Errorf("expected validation code, got %s (msg=%s)", aErr.Code, aErr.Message)
			}
			if !strings.Contains(aErr.Message, tc.want) {
				t.Errorf("message %q does not contain %q", aErr.Message, tc.want)
			}
		})
	}
}

// Image cap is strictly tighter than the attachment cap so we don't
// accidentally widen one without revisiting the other.
func TestUploadLimits_ImageStricterThanAttachment(t *testing.T) {
	t.Parallel()
	if MaxImageBytes >= MaxAttachmentBytes {
		t.Fatalf("expected MaxImageBytes(%d) < MaxAttachmentBytes(%d)", MaxImageBytes, MaxAttachmentBytes)
	}
}
