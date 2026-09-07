// Package note stores encrypted text records for a vault owner.
// The server never decrypts ciphertext; it only checks ownership, versions, and optional hashes.
package note

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
)

var (
	// ErrNotFound is a missing or foreign note.
	ErrNotFound = errors.New("note not found")
	// ErrConflict is a duplicate id or stale version.
	ErrConflict = errors.New("note conflict")
	// ErrInvalid is a request the server refuses to store.
	ErrInvalid = errors.New("invalid note")
)

// Note is one encrypted text record as stored for a user.
type Note struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	Version          int64      `db:"version"`
	Nonce            []byte     `db:"nonce"`
	Meta             string     `db:"meta"`
	Ciphertext       []byte     `db:"ciphertext"`
	CiphertextSHA256 []byte     `db:"ciphertext_sha256"`
	DeletedAt        *time.Time `db:"deleted_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
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
