package files

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
)

func TestKeyBelongsTo(t *testing.T) {
	item := uuid.New()
	other := uuid.New()
	own := newBlobObjectKey("item", item, ".png")
	cases := []struct {
		key  string
		want bool
	}{
		{own, true},
		{newBlobObjectKey("item", other, ".png"), false},
		{newBlobObjectKey("lostfound", item, ".png"), false},
		{"stash/item/" + item.String() + "/", false},
		{"stash/item/" + item.String() + "/../" + other.String() + "/x.png", false},
		{"stash/item/" + item.String() + "/sub/x.png", false},
		{"item/" + item.String() + "/x.png", false},
	}
	for _, c := range cases {
		if got := KeyBelongsTo(c.key, "item", item); got != c.want {
			t.Errorf("KeyBelongsTo(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestInlineSafe(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "IMAGE/GIF", "image/webp; charset=binary"} {
		if !InlineSafe(ct) {
			t.Errorf("%s should be inline-safe", ct)
		}
	}
	for _, ct := range []string{"image/svg+xml", "text/html", "application/pdf", "", "image/x-icon"} {
		if InlineSafe(ct) {
			t.Errorf("%s must not be served inline", ct)
		}
	}
}

func TestRegisterRejectsForeignStorageKey(t *testing.T) {
	s := &Service{}
	item := uuid.New()
	_, err := s.Register(context.Background(), RegisterInput{
		EntityType: "item",
		EntityID:   item,
		FileType:   domain.FileTypeReceipt,
		StorageKey: newBlobObjectKey("lostfound", uuid.New(), ".pdf"),
	})
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("want ownership error, got %v", err)
	}
}

func TestUploadRejectsImagesThatAreNotRaster(t *testing.T) {
	s := &Service{}
	for name, body := range map[string]string{
		"svg":  `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`,
		"html": `<!doctype html><script>alert(1)</script>`,
	} {
		_, err := s.Upload(context.Background(), UploadInput{
			EntityType:       "item",
			EntityID:         uuid.New(),
			FileType:         domain.FileTypeImage,
			OriginalFilename: "photo.png",
			ContentType:      "image/png",
			Body:             strings.NewReader(body),
			Size:             int64(len(body)),
		})
		if err == nil || !strings.Contains(err.Error(), "images must be") {
			t.Errorf("%s: want rejection, got %v", name, err)
		}
	}
}

func TestPresignRejectsUnknownEntity(t *testing.T) {
	s := &Service{}
	_, err := s.PresignUpload(context.Background(), PresignUploadInput{
		EntityType: "user", EntityID: uuid.New(), FileType: domain.FileTypeImage, OriginalFilename: "a.png",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported entity_type") {
		t.Fatalf("want unsupported entity_type, got %v", err)
	}
}
