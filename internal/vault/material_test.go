package vault_test

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"testing"

	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
)

func TestNewMaterial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		params   vault.Params
		wantErr  string
	}{
		{
			name:     "ok",
			password: "secret",
			params:   vault.DefaultParams(),
		},
		{
			name:     "empty password",
			password: "",
			params:   vault.DefaultParams(),
			wantErr:  "password is required",
		},
		{
			name:     "unsupported version",
			password: "secret",
			params:   vault.Params{Memory: 1, Iterations: 1, Parallelism: 1, Version: 1},
			wantErr:  "unsupported argon2 version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := vault.NewMaterial(tc.password, tc.params)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, m.Salt, vault.SaltSize)
			require.Len(t, m.KeyHash, sha256.Size)
			require.GreaterOrEqual(t, len(m.ProtectedKey), vault.NonceSize+sha256.Size)

			kek := argon2.IDKey([]byte(tc.password), m.Salt, tc.params.Iterations, tc.params.Memory, tc.params.Parallelism, vault.KEKSize)
			block, err := aes.NewCipher(kek)
			require.NoError(t, err)
			gcm, err := cipher.NewGCM(block)
			require.NoError(t, err)
			nonce, ct := m.ProtectedKey[:vault.NonceSize], m.ProtectedKey[vault.NonceSize:]
			vk, err := gcm.Open(nil, nonce, ct, nil)
			require.NoError(t, err)
			sum := sha256.Sum256(vk)
			require.Equal(t, m.KeyHash, sum[:])

			opened, err := vault.Unwrap(tc.password, m, tc.params)
			require.NoError(t, err)
			require.Equal(t, vk, opened)
		})
	}
}

func TestUnwrap(t *testing.T) {
	t.Parallel()

	p := vault.DefaultParams()
	m, err := vault.NewMaterial("secret", p)
	require.NoError(t, err)
	badHash := append([]byte(nil), m.KeyHash...)
	badHash[0] ^= 1

	tests := []struct {
		name     string
		password string
		material vault.Material
		params   vault.Params
		wantErr  string
	}{
		{
			name:     "ok",
			password: "secret",
			material: m,
			params:   p,
		},
		{
			name:     "wrong password",
			password: "nope",
			material: m,
			params:   p,
			wantErr:  "unwrap vault key",
		},
		{
			name:     "empty password",
			password: "",
			material: m,
			params:   p,
			wantErr:  "password is required",
		},
		{
			name:     "hash mismatch",
			password: "secret",
			material: vault.Material{Salt: m.Salt, ProtectedKey: m.ProtectedKey, KeyHash: badHash},
			params:   p,
			wantErr:  "vault key hash mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := vault.Unwrap(tc.password, tc.material, tc.params)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSealOpen(t *testing.T) {
	t.Parallel()

	key := make([]byte, vault.VKSize)
	for i := range key {
		key[i] = byte(i + 1)
	}

	tests := []struct {
		name      string
		plaintext []byte
		wantErr   string
	}{
		{name: "roundtrip", plaintext: []byte("hello")},
		{name: "too large", plaintext: make([]byte, vault.MaxPlaintext+1), wantErr: "plaintext is too large"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n, c, err := vault.Seal(key, tc.plaintext)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			plain, err := vault.Open(key, append(n, c...))
			require.NoError(t, err)
			require.Equal(t, tc.plaintext, plain)
		})
	}
}
