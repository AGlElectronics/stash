package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// MaxImageBytes is the per-image upload ceiling. Kept conservative so the
// nginx proxy (`client_max_body_size 50m`) and Go's multipart parsing both
// stay happy.
const MaxImageBytes = 25 * 1024 * 1024

// MaxAttachmentBytes covers non-image attachments (receipts, manuals, ...).
const MaxAttachmentBytes = 50 * 1024 * 1024

const presignExpiry = 15 * time.Minute

// storageRootPrefix is the single top-level folder in the bucket for new
// uploads. The MinIO/S3 console then shows one tidy root (stash/…) instead
// of many sibling prefixes (item/, signout/, …). Metadata rows that still
// point at legacy keys without this prefix continue to work.
const storageRootPrefix = "stash"

func newBlobObjectKey(entityType string, entityID uuid.UUID, ext string) string {
	t := strings.TrimSpace(strings.ToLower(entityType))
	if t == "" {
		t = "misc"
	}
	return fmt.Sprintf("%s/%s/%s/%s%s", storageRootPrefix, t, entityID, uuid.NewString(), ext)
}

// Service registers file metadata for an entity (item, signout, ...) and
// hands out presigned URLs for upload + download.
type Service struct {
	pool   *pgxpool.Pool
	client *Client
}

func NewService(pool *pgxpool.Pool, client *Client) *Service {
	return &Service{pool: pool, client: client}
}

type PresignUploadInput struct {
	EntityType       string
	EntityID         uuid.UUID
	FileType         domain.FileType
	OriginalFilename string
}

type PresignUploadResult struct {
	UploadURL  string `json:"upload_url"`
	StorageKey string `json:"storage_key"`
	ExpiresAt  string `json:"expires_at"`
}

func (s *Service) PresignUpload(ctx context.Context, in PresignUploadInput) (PresignUploadResult, error) {
	if !in.FileType.Valid() {
		return PresignUploadResult{}, httperror.Validation("invalid file_type", nil)
	}
	in.OriginalFilename = strings.TrimSpace(in.OriginalFilename)
	if in.OriginalFilename == "" {
		return PresignUploadResult{}, httperror.Validation("original_filename is required", nil)
	}

	ext := strings.ToLower(path.Ext(in.OriginalFilename))
	storageKey := newBlobObjectKey(in.EntityType, in.EntityID, ext)
	uploadURL, err := s.client.PresignedPut(ctx, storageKey, presignExpiry)
	if err != nil {
		return PresignUploadResult{}, httperror.Internal(err)
	}
	return PresignUploadResult{
		UploadURL:  uploadURL,
		StorageKey: storageKey,
		ExpiresAt:  time.Now().Add(presignExpiry).UTC().Format(time.RFC3339),
	}, nil
}

type RegisterInput struct {
	EntityType       string
	EntityID         uuid.UUID
	FileType         domain.FileType
	StorageKey       string
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	SetAsPrimary     bool
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (sqlcgen.File, error) {
	if !in.FileType.Valid() {
		return sqlcgen.File{}, httperror.Validation("invalid file_type", nil)
	}
	if in.StorageKey == "" {
		return sqlcgen.File{}, httperror.Validation("storage_key is required", nil)
	}
	if in.SizeBytes < 0 {
		return sqlcgen.File{}, httperror.Validation("size_bytes must be >= 0", nil)
	}
	if in.ContentType == "" {
		in.ContentType = "application/octet-stream"
	}

	var out sqlcgen.File
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		row, err := qtx.InsertFile(ctx, sqlcgen.InsertFileParams{
			EntityType:       in.EntityType,
			EntityID:         in.EntityID,
			FileType:         string(in.FileType),
			Bucket:           s.client.Bucket(),
			StorageKey:       in.StorageKey,
			OriginalFilename: in.OriginalFilename,
			ContentType:      in.ContentType,
			SizeBytes:        in.SizeBytes,
			UploadedBy:       auth.ActorID(ctx),
		})
		if err != nil {
			return httperror.Internal(err)
		}
		out = row

		if in.SetAsPrimary && in.EntityType == "item" && in.FileType == domain.FileTypeImage {
			if err := qtx.SetItemPrimaryImage(ctx, sqlcgen.SetItemPrimaryImageParams{
				ID:                 in.EntityID,
				PrimaryImageFileID: &row.ID,
			}); err != nil {
				return httperror.Internal(err)
			}
		}

		return audit.Record(ctx, qtx, "file.register", "file", row.ID, nil, row, "")
	})
	return out, err
}

// UploadInput is what the multipart handler hands to Service.Upload.
type UploadInput struct {
	EntityType       string
	EntityID         uuid.UUID
	FileType         domain.FileType
	OriginalFilename string
	ContentType      string
	SetAsPrimary     bool
	Body             io.Reader
	Size             int64 // bytes, 0/-1 if unknown
}

// Upload streams bytes to object storage and inserts the metadata row in a
// single backend call. This avoids the browser-direct presigned PUT (which
// requires the storage host to allow CORS, and is awkward in dev) so any
// authenticated client can attach files via a normal multipart POST.
func (s *Service) Upload(ctx context.Context, in UploadInput) (sqlcgen.File, error) {
	if !in.FileType.Valid() {
		return sqlcgen.File{}, httperror.Validation("invalid file_type", nil)
	}
	in.OriginalFilename = strings.TrimSpace(in.OriginalFilename)
	if in.OriginalFilename == "" {
		return sqlcgen.File{}, httperror.Validation("original_filename is required", nil)
	}
	if in.Body == nil {
		return sqlcgen.File{}, httperror.Validation("file body is required", nil)
	}
	limit := int64(MaxAttachmentBytes)
	if in.FileType == domain.FileTypeImage {
		limit = MaxImageBytes
	}
	if in.Size > limit {
		return sqlcgen.File{}, httperror.Validation(fmt.Sprintf("file too large (max %d bytes)", limit), nil)
	}
	if in.ContentType == "" {
		in.ContentType = "application/octet-stream"
	}
	if in.FileType == domain.FileTypeImage && !strings.HasPrefix(in.ContentType, "image/") {
		return sqlcgen.File{}, httperror.Validation("file_type=image requires an image/* content_type", nil)
	}

	ext := strings.ToLower(path.Ext(in.OriginalFilename))
	storageKey := newBlobObjectKey(in.EntityType, in.EntityID, ext)

	// Wrap with a hard limit so a lying Content-Length can't blow past the cap.
	body := io.LimitReader(in.Body, limit+1)
	if err := s.client.PutObject(ctx, storageKey, body, in.Size, in.ContentType); err != nil {
		return sqlcgen.File{}, httperror.Internal(err)
	}

	row, err := s.Register(ctx, RegisterInput{
		EntityType:       in.EntityType,
		EntityID:         in.EntityID,
		FileType:         in.FileType,
		StorageKey:       storageKey,
		OriginalFilename: in.OriginalFilename,
		ContentType:      in.ContentType,
		SizeBytes:        in.Size,
		SetAsPrimary:     in.SetAsPrimary,
	})
	if err != nil {
		// best-effort cleanup so we don't leave orphaned objects in MinIO
		_ = s.client.Remove(ctx, storageKey)
		return sqlcgen.File{}, err
	}
	return row, nil
}

// SetItemPrimaryImage flips an item's primary_image_file_id to the given
// file id (or clears it when fileID is nil). Validates that the file belongs
// to that item and is an image.
func (s *Service) SetItemPrimaryImage(ctx context.Context, itemID uuid.UUID, fileID *uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)

		if fileID != nil {
			row, err := qtx.GetFile(ctx, *fileID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return httperror.NotFound("file")
				}
				return httperror.Internal(err)
			}
			if row.EntityType != "item" || row.EntityID != itemID {
				return httperror.Validation("file does not belong to this item", nil)
			}
			if row.FileType != string(domain.FileTypeImage) {
				return httperror.Validation("only image files can be set as primary", nil)
			}
		}

		if err := qtx.SetItemPrimaryImage(ctx, sqlcgen.SetItemPrimaryImageParams{
			ID:                 itemID,
			PrimaryImageFileID: fileID,
		}); err != nil {
			return httperror.Internal(err)
		}

		var fileVal any
		if fileID != nil {
			fileVal = fileID.String()
		}
		return audit.Record(ctx, qtx, "item.primary_image_set", "item", itemID, nil,
			map[string]any{"primary_image_file_id": fileVal}, "")
	})
}

type ListFilesInput struct {
	EntityType string
	EntityID   uuid.UUID
	FileType   string
}

func (s *Service) List(ctx context.Context, in ListFilesInput) ([]sqlcgen.File, error) {
	q := sqlcgen.New(s.pool)
	rows, err := q.ListFilesForEntity(ctx, sqlcgen.ListFilesForEntityParams{
		EntityType: in.EntityType,
		EntityID:   in.EntityID,
		Column3:    in.FileType,
	})
	if err != nil {
		return nil, httperror.Internal(err)
	}
	return rows, nil
}

type DownloadResult struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// Get returns the metadata row for a file.
func (s *Service) Get(ctx context.Context, fileID uuid.UUID) (sqlcgen.File, error) {
	row, err := sqlcgen.New(s.pool).GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.File{}, httperror.NotFound("file")
		}
		return sqlcgen.File{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *Service) Download(ctx context.Context, fileID uuid.UUID) (DownloadResult, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DownloadResult{}, httperror.NotFound("file")
		}
		return DownloadResult{}, httperror.Internal(err)
	}
	url, err := s.client.PresignedGet(ctx, row.StorageKey, presignExpiry)
	if err != nil {
		return DownloadResult{}, httperror.Internal(err)
	}
	return DownloadResult{
		URL:       url,
		ExpiresAt: time.Now().Add(presignExpiry).UTC().Format(time.RFC3339),
	}, nil
}

// RawStream returns the metadata row plus a reader for the underlying object.
// The caller is responsible for closing the reader and writing the bytes to
// the response. We expose this (rather than only presigned URLs) so the
// browser never has to talk to MinIO directly — much friendlier in dev where
// the storage host is on a different origin and CORS / signed-host headers
// would otherwise bite.
func (s *Service) RawStream(ctx context.Context, fileID uuid.UUID) (sqlcgen.File, io.ReadCloser, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.File{}, nil, httperror.NotFound("file")
		}
		return sqlcgen.File{}, nil, httperror.Internal(err)
	}
	rc, err := s.client.GetObject(ctx, row.StorageKey)
	if err != nil {
		return sqlcgen.File{}, nil, httperror.Internal(err)
	}
	return row, rc, nil
}

func (s *Service) Delete(ctx context.Context, fileID uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		row, err := qtx.GetFile(ctx, fileID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("file")
			}
			return httperror.Internal(err)
		}
		if err := qtx.DeleteFile(ctx, fileID); err != nil {
			return httperror.Internal(err)
		}
		// best-effort remove from object storage
		_ = s.client.Remove(ctx, row.StorageKey)
		return audit.Record(ctx, qtx, "file.delete", "file", fileID, row, nil, "")
	})
}
