package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	// SaltSize is Argon2id salt length.
	SaltSize = 16
	// VKSize is vault key length (AES-256).
	VKSize = 32
	// KEKSize is the password-derived wrapping key length.
	KEKSize = 32
	// NonceSize is AES-GCM nonce length.
	NonceSize = 12
	// MaxPlaintext is the largest note body the client may seal (1 MiB).
	MaxPlaintext = 1 << 20

	// OWASP baseline for Argon2id (KiB / rounds / threads).
	DefaultMemory      = 19456
	DefaultIterations  = 2
	DefaultParallelism = 1
	DefaultVersion     = 19
)

// Params is the public Argon2id recipe stored per user.
type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	Version     uint8
}

func DefaultParams() Params {
	return Params{
		Memory:      DefaultMemory,
		Iterations:  DefaultIterations,
		Parallelism: DefaultParallelism,
		Version:     DefaultVersion,
	}
}

// Material is what the client sends on register and the server stores/returns.
type Material struct {
	Salt         []byte
	ProtectedKey []byte
	KeyHash      []byte
}

// NewMaterial derives KEK from password, wraps a fresh vault key, returns blobs for RegisterReq.
func NewMaterial(password string, p Params) (Material, error) {
	if password == "" {
		return Material{}, fmt.Errorf("password is required")
	}
	if p.Version != DefaultVersion {
		return Material{}, fmt.Errorf("unsupported argon2 version %d", p.Version)
	}

	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return Material{}, fmt.Errorf("salt: %w", err)
	}

	vk := make([]byte, VKSize)
	if _, err := io.ReadFull(rand.Reader, vk); err != nil {
		return Material{}, fmt.Errorf("vault key: %w", err)
	}

	kek := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, KEKSize)

	protected, err := seal(kek, vk)
	if err != nil {
		return Material{}, err
	}

	sum := sha256.Sum256(vk)
	return Material{
		Salt:         salt,
		ProtectedKey: protected,
		KeyHash:      sum[:],
	}, nil
}

// Unwrap derives KEK from password and opens ProtectedKey. Verifies KeyHash when set.
func Unwrap(password string, m Material, p Params) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if p.Version != DefaultVersion {
		return nil, fmt.Errorf("unsupported argon2 version %d", p.Version)
	}
	if len(m.Salt) != SaltSize {
		return nil, fmt.Errorf("kdf salt must be %d bytes", SaltSize)
	}
	kek := argon2.IDKey([]byte(password), m.Salt, p.Iterations, p.Memory, p.Parallelism, KEKSize)
	vk, err := Open(kek, m.ProtectedKey)
	if err != nil {
		return nil, fmt.Errorf("unwrap vault key: %w", err)
	}
	if len(m.KeyHash) > 0 {
		sum := sha256.Sum256(vk)
		if subtle.ConstantTimeCompare(sum[:], m.KeyHash) != 1 {
			return nil, fmt.Errorf("vault key hash mismatch")
		}
	}
	return vk, nil
}

// Seal encrypts plaintext with AES-256-GCM and returns nonce and ciphertext separately.
func Seal(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	if len(plaintext) > MaxPlaintext {
		return nil, nil, fmt.Errorf("plaintext is too large")
	}
	sealed, err := seal(key, plaintext)
	if err != nil {
		return nil, nil, err
	}
	return sealed[:NonceSize], sealed[NonceSize:], nil
}

// Open decrypts nonce||ciphertext produced by seal or ProtectedKey wrapping.
func Open(key, sealed []byte) ([]byte, error) {
	if len(sealed) < NonceSize+16 {
		return nil, fmt.Errorf("sealed blob is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce, ct := sealed[:NonceSize], sealed[NonceSize:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return plain, nil
}

func seal(kek, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return append(nonce, gcm.Seal(nil, nonce, plaintext, nil)...), nil
}
