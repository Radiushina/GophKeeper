package file

import (
	"context"
	"sync"
	"time"

	"github.com/Radiushina/GophKeeper/internal/blob"
	"github.com/google/uuid"
)

// MemRepo is an in-memory file metadata store for tests.
type MemRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]File
}

// NewMemRepo builds an empty memory repository.
func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]File{}}
}

// NewMemHandler is a files HTTP handler backed by memory (tests).
func NewMemHandler() *Handler {
	return NewHandler(NewService(NewMemRepo(), blob.NewMem()), nil)
}

// Create inserts metadata. Duplicate id is ErrConflict.
func (m *MemRepo) Create(_ context.Context, n File) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[n.ID]; ok {
		return File{}, ErrConflict
	}
	n.CreatedAt = time.Now().UTC()
	n.UpdatedAt = n.CreatedAt
	m.byID[n.ID] = n
	return n, nil
}

// Update replaces metadata when version matches.
func (m *MemRepo) Update(_ context.Context, n File) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[n.ID]
	if !ok || cur.UserID != n.UserID || cur.DeletedAt != nil {
		return File{}, ErrNotFound
	}
	if cur.Version != n.Version {
		return File{}, ErrConflict
	}
	cur.Version++
	cur.Nonce = n.Nonce
	cur.Meta = n.Meta
	cur.Name = n.Name
	cur.ChunkCount = n.ChunkCount
	cur.ByteSize = n.ByteSize
	cur.CiphertextSHA256 = n.CiphertextSHA256
	cur.UpdatedAt = time.Now().UTC()
	m.byID[n.ID] = cur
	return cur, nil
}

// SoftDelete writes a tombstone.
func (m *MemRepo) SoftDelete(_ context.Context, userID, id uuid.UUID) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[id]
	if !ok || cur.UserID != userID {
		return File{}, ErrNotFound
	}
	if cur.DeletedAt == nil {
		now := time.Now().UTC()
		cur.DeletedAt = &now
		cur.UpdatedAt = now
		cur.Version++
		m.byID[id] = cur
	}
	return cur, nil
}

// Get loads metadata owned by userID.
func (m *MemRepo) Get(_ context.Context, userID, id uuid.UUID) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[id]
	if !ok || cur.UserID != userID {
		return File{}, ErrNotFound
	}
	return cur, nil
}

// List returns the owner's files.
func (m *MemRepo) List(_ context.Context, userID uuid.UUID, since *time.Time) ([]File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []File
	for _, n := range m.byID {
		if n.UserID != userID {
			continue
		}
		if since == nil && n.DeletedAt != nil {
			continue
		}
		if since != nil && n.UpdatedAt.Before(*since) {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}
