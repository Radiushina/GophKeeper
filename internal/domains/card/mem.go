package card

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemRepo is an in-memory cards store for tests.
type MemRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]Card
}

// NewMemRepo builds an empty memory repository.
func NewMemRepo() *MemRepo {
	return &MemRepo{byID: map[uuid.UUID]Card{}}
}

// NewMemHandler is a cards HTTP handler backed by memory (tests).
func NewMemHandler() *Handler {
	return NewHandler(NewService(NewMemRepo()), nil)
}

// Create inserts a card. Duplicate id is ErrConflict.
func (m *MemRepo) Create(_ context.Context, n Card) (Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[n.ID]; ok {
		return Card{}, ErrConflict
	}
	n.CreatedAt = time.Now().UTC()
	n.UpdatedAt = n.CreatedAt
	m.byID[n.ID] = n
	return n, nil
}

// Update applies a new blob when version matches and the row is live.
func (m *MemRepo) Update(_ context.Context, n Card) (Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[n.ID]
	if !ok || cur.UserID != n.UserID || cur.DeletedAt != nil {
		return Card{}, ErrNotFound
	}
	if cur.Version != n.Version {
		return Card{}, ErrConflict
	}
	cur.Version++
	cur.Nonce = n.Nonce
	cur.Meta = n.Meta
	cur.Ciphertext = n.Ciphertext
	cur.CiphertextSHA256 = n.CiphertextSHA256
	cur.UpdatedAt = time.Now().UTC()
	m.byID[n.ID] = cur
	return cur, nil
}

// SoftDelete sets deleted_at. A second call returns the existing tombstone.
func (m *MemRepo) SoftDelete(_ context.Context, userID, id uuid.UUID) (Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[id]
	if !ok || cur.UserID != userID {
		return Card{}, ErrNotFound
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

// Get loads a card owned by userID.
func (m *MemRepo) Get(_ context.Context, userID, id uuid.UUID) (Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.byID[id]
	if !ok || cur.UserID != userID {
		return Card{}, ErrNotFound
	}
	return cur, nil
}

// List returns the owner's cards. A nil since means live rows only.
func (m *MemRepo) List(_ context.Context, userID uuid.UUID, since *time.Time) ([]Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Card
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
