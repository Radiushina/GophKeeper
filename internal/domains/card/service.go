package card

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo RepoProvider
}

type RepoProvider interface {
	Create(ctx context.Context, n Card) (Card, error)
	Update(ctx context.Context, n Card) (Card, error)
	SoftDelete(ctx context.Context, userID, id uuid.UUID) (Card, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Card, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Card, error)
}

func NewService(repo RepoProvider) *Service {
	return &Service{repo: repo}
}

// Create stores a new encrypted card. Version must be 1.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Card, error) {
	if userID == uuid.Nil {
		return Card{}, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	if in.ID == uuid.Nil {
		return Card{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if in.Version != CreateVersion {
		return Card{}, fmt.Errorf("%w: create version must be %d", ErrConflict, CreateVersion)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return Card{}, err
	}
	if err := validateMeta(in.Meta); err != nil {
		return Card{}, err
	}
	created, err := s.repo.Create(ctx, Card{
		ID:               in.ID,
		UserID:           userID,
		Version:          CreateVersion,
		Nonce:            append([]byte(nil), in.Nonce...),
		Meta:             in.Meta,
		Ciphertext:       append([]byte(nil), in.Ciphertext...),
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		return Card{}, fmt.Errorf("create card: %w", err)
	}
	return created, nil
}

// Update replaces ciphertext when version matches the stored row.
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (Card, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Card{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return Card{}, err
	}
	if err := validateMeta(in.Meta); err != nil {
		return Card{}, err
	}
	updated, err := s.repo.Update(ctx, Card{
		ID:               id,
		UserID:           userID,
		Version:          in.Version,
		Nonce:            append([]byte(nil), in.Nonce...),
		Meta:             in.Meta,
		Ciphertext:       append([]byte(nil), in.Ciphertext...),
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		return Card{}, fmt.Errorf("update card: %w", err)
	}
	return updated, nil
}

// Delete writes a tombstone. Repeating the call returns the same tombstone.
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) (Card, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Card{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.SoftDelete(ctx, userID, id)
	if err != nil {
		return Card{}, fmt.Errorf("delete card: %w", err)
	}
	return n, nil
}

// Get returns a card owned by userID, including tombstones.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Card, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Card{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return Card{}, fmt.Errorf("get card: %w", err)
	}
	return n, nil
}

// List returns live cards, or changes since the cursor (including tombstones).
func (s *Service) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Card, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	items, err := s.repo.List(ctx, userID, since)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	if items == nil {
		return []Card{}, nil
	}
	return items, nil
}

func cloneHash(sum []byte) []byte {
	if len(sum) == 0 {
		return nil
	}
	return append([]byte(nil), sum...)
}
