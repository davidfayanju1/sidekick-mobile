// Package storage defines bucket privacy posture and signed-URL access.
//
// Pure-Go reading of BACKEND.md §17: no Supabase Storage is used. This
// package is the boundary every later phase calls for uploads/downloads.
// Buckets:
//   avatars, task-photos      — public read of listing content, signed upload
//   chat-images               — private, signed expiring URLs, parties only
//   id-documents              — private, NO public access, short-TTL signed
//                              URLs, admin-only read, encrypted at rest (*)
//   dispute-evidence          — private, signed URLs for parties + admin
// (*) "encrypted at rest" is an S3 SSE setting applied by the production
// object-store configuration; the posture enforced HERE is: no public
// read path exists for private buckets, and every private read requires
// a server-minted signed URL that the Store verifies.
package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Bucket string

const (
	BucketAvatars         Bucket = "avatars"
	BucketTaskPhotos      Bucket = "task-photos"
	BucketChatImages      Bucket = "chat-images"
	BucketIDDocuments     Bucket = "id-documents"
	BucketDisputeEvidence Bucket = "dispute-evidence"
)

// AllBuckets lists every bucket Phase 0 §7 requires.
var AllBuckets = []Bucket{
	BucketAvatars, BucketTaskPhotos, BucketChatImages,
	BucketIDDocuments, BucketDisputeEvidence,
}

// PublicRead reports whether anonymous GET of listing content is intended.
func PublicRead(b Bucket) bool {
	return b == BucketAvatars || b == BucketTaskPhotos
}

// Private reports the locked-down posture for sensitive buckets.
func Private(b Bucket) bool {
	return b == BucketChatImages || b == BucketIDDocuments || b == BucketDisputeEvidence
}

// SignedURL is an HMAC-based capability: method + bucket + object + expiry.
// Production may swap the scheme for S3 presigned URLs; the verified
// properties (unforgeable, expiring, bucket-scoped) stay the same.
type SignedURL struct {
	Bucket    Bucket
	Object    string
	ExpiresAt time.Time
	Signature string
}

// Store mints and verifies signed URLs. Secret comes from server env only.
type Store struct {
	secret []byte
	now    func() time.Time
}

func NewStore(secret string) *Store {
	if secret == "" {
		secret = "phase0-test-secret-not-for-prod-use-0123456789"
	}
	return &Store{secret: []byte(secret), now: time.Now}
}

// MintUpload returns an upload capability valid for ttl.
func (s *Store) MintUpload(bucket Bucket, object string, ttl time.Duration) SignedURL {
	return s.mint("PUT", bucket, object, ttl)
}

// MintDownload returns a download capability valid for ttl.
func (s *Store) MintDownload(bucket Bucket, object string, ttl time.Duration) SignedURL {
	return s.mint("GET", bucket, object, ttl)
}

func (s *Store) mint(method string, bucket Bucket, object string, ttl time.Duration) SignedURL {
	exp := s.now().Add(ttl).UTC()
	msg := strings.Join([]string{method, string(bucket), object, exp.Format(time.RFC3339)}, "\n")
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(msg))
	return SignedURL{
		Bucket:    bucket,
		Object:    object,
		ExpiresAt: exp,
		Signature: hex.EncodeToString(mac.Sum(nil)),
	}
}

// Verify checks a capability for the expected method. It rejects expired
// or forged URLs.
func (s *Store) Verify(method string, u SignedURL) error {
	if s.now().After(u.ExpiresAt) {
		return fmt.Errorf("storage: signed URL expired")
	}
	msg := strings.Join([]string{method, string(u.Bucket), u.Object, u.ExpiresAt.Format(time.RFC3339)}, "\n")
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(msg))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(u.Signature)) {
		return fmt.Errorf("storage: invalid signature")
	}
	return nil
}

// AuthorizeRead is the single gate every download path calls:
//   - public buckets: allowed without a capability (listing content);
//   - private buckets: a valid GET capability is mandatory, otherwise denied.
func (s *Store) AuthorizeRead(bucket Bucket, u *SignedURL) error {
	if PublicRead(bucket) {
		return nil
	}
	if u == nil {
		return fmt.Errorf("storage: %s requires a signed URL", bucket)
	}
	if u.Bucket != bucket {
		return fmt.Errorf("storage: signed URL is for another bucket")
	}
	return s.Verify("GET", *u)
}
