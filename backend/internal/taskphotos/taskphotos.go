// Package taskphotos handles listing-photo uploads (§3.3): signed grants
// into task-photos/{task_id}/..., 10MB cap, jpg/png/webp whitelist, and
// the server-side 5-listing-photos cap (enforced in tasks.AttachPhoto,
// never client-side). Completion photos are linkable already (0–5) for
// Phase 6 — no schema change needed later.
package taskphotos

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/storage"
)

const MaxBytes = 10 * 1024 * 1024

var (
	ErrTooLarge = errors.New("photo exceeds 10MB cap")
	ErrBadType  = errors.New("photo must be jpg, png or webp")
)

var allowContent = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true,
}

type Service struct {
	signer *storage.Store
	blobs  *blobstore.MemoryBlobStore
}

func NewService(signer *storage.Store, blobs *blobstore.MemoryBlobStore) *Service {
	if blobs == nil {
		blobs = blobstore.NewMemoryBlobStore()
	}
	return &Service{signer: signer, blobs: blobs}
}

type UploadGrant struct {
	Object    string
	ExpiresAt time.Time
	Signature string
}

func (s *Service) GrantUpload(taskID, contentType string) (UploadGrant, error) {
	if !allowContent[strings.ToLower(contentType)] {
		return UploadGrant{}, ErrBadType
	}
	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}
	object := path.Join(taskID, uuid.NewString()+ext)
	cap := s.signer.MintUpload(storage.BucketTaskPhotos, object, 15*time.Minute)
	return UploadGrant{Object: object, ExpiresAt: cap.ExpiresAt, Signature: cap.Signature}, nil
}

// Complete verifies the grant, validates size/type/decodability and stores.
// Returns the storage key to persist as task_photos.url.
func (s *Service) Complete(grant UploadGrant, data []byte) (string, error) {
	if err := s.signer.Verify("PUT", storage.SignedURL{
		Bucket: storage.BucketTaskPhotos, Object: grant.Object,
		ExpiresAt: grant.ExpiresAt, Signature: grant.Signature,
	}); err != nil {
		return "", fmt.Errorf("task-photos: %w", err)
	}
	if len(data) > MaxBytes {
		return "", ErrTooLarge
	}
	if ct := http.DetectContentType(data); !allowContent[ct] {
		// DetectContentType never reports webp reliably; try decode as fallback.
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			return "", fmt.Errorf("%w: %v", ErrBadType, err)
		}
	} else if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadType, err)
	}
	key := path.Join(string(storage.BucketTaskPhotos), grant.Object)
	s.blobs.Put(key, data, http.DetectContentType(data))
	return key, nil
}
