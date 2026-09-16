package file

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Radiushina/GophKeeper/internal/blob"
	"github.com/google/uuid"
)

// Service applies file rules and talks to Postgres + object storage.
type Service struct {
	repo  RepoProvider
	blobs blob.Store
}

// RepoProvider is the persistence port for file metadata.
type RepoProvider interface {
	Create(ctx context.Context, n File) (File, error)
	Update(ctx context.Context, n File) (File, error)
	SoftDelete(ctx context.Context, userID, id uuid.UUID) (File, error)
	Get(ctx context.Context, userID, id uuid.UUID) (File, error)
	List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]File, error)
}

// NewService wires a repository and blob store into the file service.
func NewService(repo RepoProvider, blobs blob.Store) *Service {
	return &Service{repo: repo, blobs: blobs}
}

// Create stores metadata in Postgres and ciphertext in S3. Version must be 1.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (File, error) {
	if userID == uuid.Nil {
		return File{}, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	if in.ID == uuid.Nil {
		return File{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if in.Version != CreateVersion {
		return File{}, fmt.Errorf("%w: create version must be %d", ErrConflict, CreateVersion)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return File{}, err
	}
	if err := validateMeta(in.Meta); err != nil {
		return File{}, err
	}
	key := ObjectKey(userID, in.ID)
	if err := s.blobs.Put(ctx, key, in.Ciphertext); err != nil {
		return File{}, fmt.Errorf("put file blob: %w", err)
	}
	created, err := s.repo.Create(ctx, File{
		ID:               in.ID,
		UserID:           userID,
		Version:          CreateVersion,
		Nonce:            append([]byte(nil), in.Nonce...),
		Meta:             in.Meta,
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		_ = s.blobs.Delete(ctx, key)
		return File{}, fmt.Errorf("create file: %w", err)
	}
	created.Ciphertext = append([]byte(nil), in.Ciphertext...)
	return created, nil
}

// Update replaces ciphertext when version matches the stored row.
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (File, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return File{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if err := validateBlob(in.Version, in.Nonce, in.Ciphertext, in.CiphertextSHA256); err != nil {
		return File{}, err
	}
	if err := validateMeta(in.Meta); err != nil {
		return File{}, err
	}
	key := ObjectKey(userID, id)
	if err := s.blobs.Put(ctx, key, in.Ciphertext); err != nil {
		return File{}, fmt.Errorf("put file blob: %w", err)
	}
	updated, err := s.repo.Update(ctx, File{
		ID:               id,
		UserID:           userID,
		Version:          in.Version,
		Nonce:            append([]byte(nil), in.Nonce...),
		Meta:             in.Meta,
		CiphertextSHA256: cloneHash(in.CiphertextSHA256),
	})
	if err != nil {
		return File{}, fmt.Errorf("update file: %w", err)
	}
	updated.Ciphertext = append([]byte(nil), in.Ciphertext...)
	return updated, nil
}

// Delete writes a tombstone and drops the S3 object.
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) (File, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return File{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.SoftDelete(ctx, userID, id)
	if err != nil {
		return File{}, fmt.Errorf("delete file: %w", err)
	}
	_ = s.blobs.Delete(ctx, ObjectKey(userID, id))
	if keys, listErr := s.blobs.List(ctx, ChunkPrefix(userID, id)); listErr == nil {
		for _, k := range keys {
			_ = s.blobs.Delete(ctx, k)
		}
	}
	return n, nil
}

// Get returns a file owned by userID, including tombstones.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (File, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return File{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	n, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return File{}, fmt.Errorf("get file: %w", err)
	}
	if n.ChunkCount > 0 {
		return n, nil
	}
	return s.attachBlob(ctx, n)
}

// List returns live files, or changes since the cursor (including tombstones).
// Chunked files are metadata-only so a list cannot OOM on multi-gigabyte blobs.
func (s *Service) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]File, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing owner", ErrInvalid)
	}
	items, err := s.repo.List(ctx, userID, since)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	if items == nil {
		return []File{}, nil
	}
	return items, nil
}

// UploadHeader is the first gRPC upload message.
type UploadHeader struct {
	ID      uuid.UUID
	Version int64
	Name    string
	Meta    string
	Size    int64
}

// PutChunk stores one sealed chunk. index is 0-based.
func (s *Service) PutChunk(ctx context.Context, userID, id uuid.UUID, index int32, nonce, ciphertext []byte) error {
	if userID == uuid.Nil || id == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if index < 0 {
		return fmt.Errorf("%w: chunk index", ErrInvalid)
	}
	if err := validateBlob(1, nonce, ciphertext, nil); err != nil {
		return err
	}
	if err := s.blobs.Put(ctx, ChunkKey(userID, id, index), append(append([]byte(nil), nonce...), ciphertext...)); err != nil {
		return fmt.Errorf("put chunk: %w", err)
	}
	return nil
}

// ChunkStatus is how many chunks are already in S3 (for resume).
type ChunkStatus struct {
	NextIndex int32
	Chunks    int32
}

// UploadStatus counts stored chunks for resume-after-disconnect.
func (s *Service) UploadStatus(ctx context.Context, userID, id uuid.UUID) (ChunkStatus, error) {
	if userID == uuid.Nil || id == uuid.Nil {
		return ChunkStatus{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	keys, err := s.blobs.List(ctx, ChunkPrefix(userID, id))
	if err != nil {
		return ChunkStatus{}, fmt.Errorf("list chunks: %w", err)
	}
	// ponytail: count keys; indexes are dense 0..n-1. Sparse holes need a map.
	n := int32(len(keys))
	return ChunkStatus{NextIndex: n, Chunks: n}, nil
}

// FinishUpload writes metadata after the last chunk. version 1 creates, else updates.
func (s *Service) FinishUpload(ctx context.Context, userID uuid.UUID, in UploadHeader, chunks int32) (File, error) {
	if userID == uuid.Nil || in.ID == uuid.Nil {
		return File{}, fmt.Errorf("%w: id is required", ErrInvalid)
	}
	if in.Size > MaxFileSize {
		return File{}, SizeError(in.Size)
	}
	if err := validateMeta(in.Meta); err != nil {
		return File{}, err
	}
	if len(in.Name) > MaxName {
		return File{}, fmt.Errorf("%w: name is too large", ErrInvalid)
	}
	placeholder := make([]byte, 12)
	row := File{
		ID:         in.ID,
		UserID:     userID,
		Version:    in.Version,
		Nonce:      placeholder,
		Meta:       in.Meta,
		Name:       in.Name,
		ChunkCount: chunks,
		ByteSize:   in.Size,
	}
	if in.Version == CreateVersion {
		created, err := s.repo.Create(ctx, row)
		if err != nil {
			return File{}, fmt.Errorf("create file: %w", err)
		}
		return created, nil
	}
	updated, err := s.repo.Update(ctx, row)
	if err != nil {
		return File{}, fmt.Errorf("update file: %w", err)
	}
	return updated, nil
}

// PatchMeta updates name/meta without touching chunks.
func (s *Service) PatchMeta(ctx context.Context, userID, id uuid.UUID, version int64, name, meta string) (File, error) {
	cur, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return File{}, fmt.Errorf("get file: %w", err)
	}
	if err := validateMeta(meta); err != nil {
		return File{}, err
	}
	if len(name) > MaxName {
		return File{}, fmt.Errorf("%w: name is too large", ErrInvalid)
	}
	cur.Version = version
	cur.Meta = meta
	if name != "" {
		cur.Name = name
	}
	updated, err := s.repo.Update(ctx, cur)
	if err != nil {
		return File{}, fmt.Errorf("update file: %w", err)
	}
	return updated, nil
}

// OpenChunk loads one sealed chunk (nonce||ciphertext).
func (s *Service) OpenChunk(ctx context.Context, userID, id uuid.UUID, index int32) ([]byte, error) {
	data, err := s.blobs.Get(ctx, ChunkKey(userID, id, index))
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, fmt.Errorf("%w: chunk %d", ErrNotFound, index)
		}
		return nil, fmt.Errorf("get chunk: %w", err)
	}
	return data, nil
}

func (s *Service) attachBlob(ctx context.Context, f File) (File, error) {
	if f.DeletedAt != nil {
		return f, nil
	}
	data, err := s.blobs.Get(ctx, ObjectKey(f.UserID, f.ID))
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return File{}, fmt.Errorf("%w: blob missing", ErrNotFound)
		}
		return File{}, fmt.Errorf("get file blob: %w", err)
	}
	f.Ciphertext = data
	return f, nil
}

func cloneHash(sum []byte) []byte {
	if len(sum) == 0 {
		return nil
	}
	return append([]byte(nil), sum...)
}
