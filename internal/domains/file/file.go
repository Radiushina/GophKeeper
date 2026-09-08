// Package file stores encrypted binary records for a vault owner.
// Ciphertext lives in object storage; Postgres keeps metadata and versions.
package file

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
)

const (
	// CreateVersion is the only version accepted on create.
	CreateVersion = 1
	// MaxCiphertext is AES-GCM ciphertext for a 1 MiB plaintext (tag included, nonce is separate).
	MaxCiphertext = vault.MaxPlaintext + 16
	// MaxMeta is the largest plaintext metadata string the server stores.
	MaxMeta = 4096
	// MaxFileSize is the largest plaintext file the server accepts (512 MiB).
	MaxFileSize = 512 << 20
	// MaxName is the largest plaintext filename.
	MaxName = 512
	// ChunkPlaintext is one sealed chunk (same cap as vault.Seal).
	ChunkPlaintext = vault.MaxPlaintext
)

var (
	// ErrNotFound is a missing or foreign file.
	ErrNotFound = errors.New("file not found")
	// ErrConflict is a duplicate id or stale version.
	ErrConflict = errors.New("file conflict")
	// ErrInvalid is a request the server refuses to store.
	ErrInvalid = errors.New("invalid file")
	// ErrTooLarge is a file over MaxFileSize.
	ErrTooLarge = errors.New("file is too large")
)

// File is one encrypted binary record as stored for a user.
type File struct {
	ID     uuid.UUID `db:"id"`
	UserID uuid.UUID `db:"user_id"`
	// Version is the optimistic-lock counter; create is 1, each write increments.
	Version int64 `db:"version"`
	// Nonce is the AES-256-GCM nonce for a single REST blob. Chunked gRPC files keep a placeholder; each chunk has its own nonce in S3.
	Nonce []byte `db:"nonce"`
	// Meta is unencrypted tags the server can list/filter without decrypting.
	Meta string `db:"meta"`
	// Ciphertext is the REST body loaded from S3, not stored in Postgres. Empty for chunked files.
	Ciphertext []byte `db:"-"`
	// CiphertextSHA256 is an optional checksum of the REST ciphertext. Unused for chunked uploads.
	CiphertextSHA256 []byte `db:"ciphertext_sha256"`
	Name             string `db:"name"`
	// ChunkCount is how many S3 chunk objects belong to this version. 0 means a single REST object at ObjectKey.
	ChunkCount int32 `db:"chunk_count"`
	// ByteSize is the plaintext size from the gRPC upload header (0 for REST creates).
	ByteSize int64 `db:"byte_size"`
	// DeletedAt is a sync tombstone; nil means the file is live.
	DeletedAt *time.Time `db:"deleted_at"`
	// CreatedAt is when the server first stored the row.
	CreatedAt time.Time `db:"created_at"`
	// UpdatedAt is when the server last wrote the row.
	UpdatedAt time.Time `db:"updated_at"`
}

// CreateInput is the client-supplied create payload.
type CreateInput struct {
	ID               uuid.UUID
	Version          int64
	Nonce            []byte
	Meta             string
	Ciphertext       []byte
	CiphertextSHA256 []byte
}

// UpdateInput is the client-supplied update payload.
type UpdateInput struct {
	Version          int64
	Nonce            []byte
	Meta             string
	Ciphertext       []byte
	CiphertextSHA256 []byte
}

// ObjectKey is the S3 object path for a file owned by userID.
func ObjectKey(userID, id uuid.UUID) string {
	return fmt.Sprintf("files/%s/%s", userID.String(), id.String())
}

// ChunkPrefix is the S3 key prefix for streamed chunks of one file.
func ChunkPrefix(userID, id uuid.UUID) string {
	return fmt.Sprintf("files/%s/%s/", userID.String(), id.String())
}

// ChunkKey is the S3 object for one sealed chunk.
func ChunkKey(userID, id uuid.UUID, index int32) string {
	return fmt.Sprintf("files/%s/%s/%d", userID.String(), id.String(), index)
}

// SizeError returns a user-facing too-large error.
func SizeError(got int64) error {
	return fmt.Errorf("%w: file is too large: %s, maximum is 512 MiB", ErrTooLarge, formatSize(got))
}

func formatSize(n int64) string {
	const miB = 1 << 20
	if n%(miB) == 0 {
		return fmt.Sprintf("%d MiB", n/miB)
	}
	return fmt.Sprintf("%.1f MiB", float64(n)/float64(miB))
}

func validateBlob(version int64, nonce, ciphertext, sum []byte) error {
	if version < 1 {
		return fmt.Errorf("%w: version must be >= 1", ErrInvalid)
	}
	if len(nonce) != vault.NonceSize {
		return fmt.Errorf("%w: nonce must be %d bytes", ErrInvalid, vault.NonceSize)
	}
	if len(ciphertext) < 16 {
		return fmt.Errorf("%w: ciphertext is too short", ErrInvalid)
	}
	if len(ciphertext) > MaxCiphertext {
		return fmt.Errorf("%w: ciphertext is too large", ErrInvalid)
	}
	if len(sum) == 0 {
		return nil
	}
	if len(sum) != sha256.Size {
		return fmt.Errorf("%w: ciphertext_sha256 must be %d bytes", ErrInvalid, sha256.Size)
	}
	got := sha256.Sum256(ciphertext)
	if subtle.ConstantTimeCompare(sum, got[:]) != 1 {
		return fmt.Errorf("%w: ciphertext_sha256 mismatch", ErrInvalid)
	}
	return nil
}

func validateMeta(meta string) error {
	if len(meta) > MaxMeta {
		return fmt.Errorf("%w: meta is too large", ErrInvalid)
	}
	return nil
}
