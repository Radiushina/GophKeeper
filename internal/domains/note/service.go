package note

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service applies note rules and talks to storage.
type Service struct {
	repo RepoProvider
}

// RepoProvider is the persistence port for notes.
type RepoProvider interface {
	Create(ctx context.Context, n Note) (Note, error)
	Update(ctx context.Context, n Note) (Note, error)
	SoftDelete(ctx context.Context, userID, id uuid.UUID) (Note, error)
	Get(ctx context.Context, userID, id uuid.UUID) (Note, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Note, error)
}

// NewService wires a repository into the note service.
func NewService(repo RepoProvider) *Service {
	return &Service{repo: repo}
}

// Create stores a new encrypted note. Version must be 1.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Note, error) {
	if userID == uuid.Nil {
		return Note{}, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	if in.ID == uuid.Nil {
		return Note{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if in.Version != CreateVersion {
		return Note{}, fmt.Errorf("%w: create version must be %d", ErrConflict, CreateVersion)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return Note{}, err
	}
	created, err := s.repo.Create(ctx, Note{
		ID:               in.ID,
		UserID:           userID,
		Version:          CreateVersion,
		Nonce:            append([]byte(nil), in.Nonce...),
		Ciphertext:       append([]byte(nil), in.Ciphertext...),
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		return Note{}, fmt.Errorf("create note: %w", err)
	}
	return created, nil
}

// Update replaces ciphertext when version matches the stored row.
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (Note, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Note{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return Note{}, err
	}
	updated, err := s.repo.Update(ctx, Note{
		ID:               id,
		UserID:           userID,
		Version:          in.Version,
		Nonce:            append([]byte(nil), in.Nonce...),
		Ciphertext:       append([]byte(nil), in.Ciphertext...),
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	return updated, nil
}

// Delete writes a tombstone. Repeating the call returns the same tombstone.
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Note{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.SoftDelete(ctx, userID, id)
	if err != nil {
		return Note{}, fmt.Errorf("delete note: %w", err)
	}
	return n, nil
}

// Get returns a note owned by userID, including tombstones.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return Note{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return Note{}, fmt.Errorf("get note: %w", err)
	}
	return n, nil
}

// List returns live notes, or changes since the cursor (including tombstones).
func (s *Service) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Note, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	items, err := s.repo.List(ctx, userID, since)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	if items == nil {
		return []Note{}, nil
	}
	return items, nil
}

func cloneHash(sum []byte) []byte {
	if len(sum) == 0 {
		return nil
	}
	return append([]byte(nil), sum...)
}
