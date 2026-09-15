// Package blobstore provides the shared in-memory blob backend for dev
// and tests. Production swaps in an S3 implementation behind the same
// BlobStore boundary; signed-URL capabilities come from storage.Store.
package blobstore

import "sync"

// BlobStore persists processed upload bytes keyed by bucket/object path.
type BlobStore interface {
	Put(key string, data []byte, contentType string)
	Get(key string) ([]byte, bool)
}

type MemoryBlobStore struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func NewMemoryBlobStore() *MemoryBlobStore { return &MemoryBlobStore{objs: map[string][]byte{}} }

func (m *MemoryBlobStore) Put(key string, data []byte, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objs[key] = data
}

func (m *MemoryBlobStore) Get(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	return b, ok
}
