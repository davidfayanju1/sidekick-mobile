// Package avatar handles profile photo upload (§2.3): signed upload URL
// into avatars/{user_id}/..., server-side resize, 5MB hard cap.
//
// Pure-Go storage: blobs live behind the BlobStore boundary (memory store
// for dev/test; S3 implementation in production). The signed URL minted by
// storage.Store authorizes the upload; Complete validates + resizes.
package avatar

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

	"github.com/disintegration/imaging"
	"github.com/google/uuid"

	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/storage"
)

const (
	MaxBytes   = 5 * 1024 * 1024
	MaxSidePx  = 512
	JPEGQuality = 85
)

var (
	ErrTooLarge = errors.New("avatar exceeds 5MB cap")
	ErrBadType  = errors.New("avatar must be jpeg, png or webp")
)

var allowContent = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true,
}

// BlobStore persists processed avatars (shared in-memory impl in blobstore).
type BlobStore = blobstore.BlobStore

// NewMemoryBlobStore returns the shared in-memory backend (dev/test).
func NewMemoryBlobStore() *blobstore.MemoryBlobStore { return blobstore.NewMemoryBlobStore() }

type Service struct {
	signer *storage.Store
	blobs  BlobStore
}

func NewService(signer *storage.Store, blobs BlobStore) *Service {
	if blobs == nil {
		blobs = NewMemoryBlobStore()
	}
	return &Service{signer: signer, blobs: blobs}
}

type UploadGrant struct {
	Object    string             `json:"object"`
	Signature storage.SignedURL `json:"-"`
	ExpiresAt time.Time          `json:"expires_at"`
}

// GrantUpload mints an upload capability for one avatar object.
func (s *Service) GrantUpload(userID, contentType string) (UploadGrant, error) {
	if !allowContent[strings.ToLower(contentType)] {
		return UploadGrant{}, ErrBadType
	}
	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}
	object := path.Join(userID, uuid.NewString()+ext)
	cap := s.signer.MintUpload(storage.BucketAvatars, object, 15*time.Minute)
	return UploadGrant{Object: object, Signature: cap, ExpiresAt: cap.ExpiresAt}, nil
}

// Complete verifies the capability, validates size/type, resizes to a
// max 512px side and stores the processed bytes. Returns the avatar key
// to persist as users.avatar_url.
func (s *Service) Complete(grant UploadGrant, data []byte) (string, error) {
	if err := s.signer.Verify("PUT", grant.Signature); err != nil {
		return "", fmt.Errorf("avatar: %w", err)
	}
	if len(data) > MaxBytes {
		return "", ErrTooLarge
	}
	ct := http.DetectContentType(data)
	if !allowContent[ct] && !strings.HasPrefix(ct, "image/") {
		return "", ErrBadType
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadType, err)
	}
	resized := imaging.Fit(img, MaxSidePx, MaxSidePx, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(JPEGQuality)); err != nil {
		return "", err
	}
	key := path.Join(string(storage.BucketAvatars), grant.Object)
	s.blobs.Put(key, buf.Bytes(), "image/jpeg")
	return key, nil
}

// Open returns stored bytes (serves public reads in dev/test).
func (s *Service) Open(key string) ([]byte, bool) { return s.blobs.Get(key) }
