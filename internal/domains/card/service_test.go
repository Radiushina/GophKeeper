package card_test

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_Create(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	id := uuid.New()
	nonce := fillBytes(vault.NonceSize, 1)
	ct := fillBytes(32, 2)
	sum := sha256.Sum256(ct)
	badSum := append([]byte(nil), sum[:]...)
	badSum[0] ^= 1

	tests := []struct {
		name    string
		owner   uuid.UUID
		in      card.CreateInput
		prepare func(t *testing.T, svc *card.Service)
		wantErr error
	}{
		{
			name:  "ok",
			owner: owner,
			in: card.CreateInput{
				ID: id, Version: card.CreateVersion, Nonce: nonce, Meta: "work", Ciphertext: ct, CiphertextSHA256: sum[:],
			},
		},
		{
			name:  "duplicate id",
			owner: owner,
			in:    card.CreateInput{ID: id, Version: 1, Nonce: nonce, Ciphertext: ct},
			prepare: func(t *testing.T, svc *card.Service) {
				t.Helper()
				_, err := svc.Create(t.Context(), owner, card.CreateInput{
					ID: id, Version: card.CreateVersion, Nonce: nonce, Ciphertext: ct, CiphertextSHA256: sum[:],
				})
				require.NoError(t, err)
			},
			wantErr: card.ErrConflict,
		},
		{
			name:    "wrong create version",
			owner:   owner,
			in:      card.CreateInput{ID: uuid.New(), Version: 2, Nonce: nonce, Ciphertext: ct},
			wantErr: card.ErrConflict,
		},
		{
			name:    "short nonce",
			owner:   owner,
			in:      card.CreateInput{ID: uuid.New(), Version: 1, Nonce: []byte{1}, Ciphertext: ct},
			wantErr: card.ErrInvalid,
		},
		{
			name:    "bad sha256",
			owner:   owner,
			in:      card.CreateInput{ID: uuid.New(), Version: 1, Nonce: nonce, Ciphertext: ct, CiphertextSHA256: badSum},
			wantErr: card.ErrInvalid,
		},
		{
			name:    "empty owner",
			in:      card.CreateInput{ID: uuid.New(), Version: 1, Nonce: nonce, Ciphertext: ct},
			wantErr: card.ErrInvalid,
		},
		{
			name:    "meta too large",
			owner:   owner,
			in:      card.CreateInput{ID: uuid.New(), Version: 1, Nonce: nonce, Meta: string(make([]byte, card.MaxMeta+1)), Ciphertext: ct},
			wantErr: card.ErrInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := card.NewService(card.NewMemRepo())
			if tc.prepare != nil {
				tc.prepare(t, svc)
			}
			got, err := svc.Create(t.Context(), tc.owner, tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.in.ID, got.ID)
			require.Equal(t, int64(1), got.Version)
			require.Equal(t, tc.in.Meta, got.Meta)
		})
	}
}

func TestService_UpdateGetDeleteList(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	id := uuid.New()
	nonce := fillBytes(vault.NonceSize, 1)
	ct := fillBytes(32, 2)
	svc := card.NewService(card.NewMemRepo())
	created, err := svc.Create(t.Context(), owner, card.CreateInput{
		ID: id, Version: card.CreateVersion, Nonce: nonce, Ciphertext: ct,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Version)

	var deleted card.Card
	steps := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "update ok",
			run: func(t *testing.T) {
				updated, err := svc.Update(t.Context(), owner, id, card.UpdateInput{
					Version: 1, Nonce: fillBytes(vault.NonceSize, 3), Ciphertext: fillBytes(32, 4),
				})
				require.NoError(t, err)
				require.Equal(t, int64(2), updated.Version)
			},
		},
		{
			name: "stale version",
			run: func(t *testing.T) {
				_, err := svc.Update(t.Context(), owner, id, card.UpdateInput{
					Version: 1, Nonce: nonce, Ciphertext: ct,
				})
				require.ErrorIs(t, err, card.ErrConflict)
			},
		},
		{
			name: "get",
			run: func(t *testing.T) {
				got, err := svc.Get(t.Context(), owner, id)
				require.NoError(t, err)
				require.Equal(t, int64(2), got.Version)
			},
		},
		{
			name: "delete",
			run: func(t *testing.T) {
				var err error
				deleted, err = svc.Delete(t.Context(), owner, id)
				require.NoError(t, err)
				require.NotNil(t, deleted.DeletedAt)
			},
		},
		{
			name: "delete idempotent",
			run: func(t *testing.T) {
				again, err := svc.Delete(t.Context(), owner, id)
				require.NoError(t, err)
				require.Equal(t, deleted.Version, again.Version)
			},
		},
		{
			name: "update deleted",
			run: func(t *testing.T) {
				_, err := svc.Update(t.Context(), owner, id, card.UpdateInput{
					Version: deleted.Version, Nonce: nonce, Ciphertext: ct,
				})
				require.ErrorIs(t, err, card.ErrNotFound)
			},
		},
		{
			name: "list live empty",
			run: func(t *testing.T) {
				live, err := svc.List(t.Context(), owner, nil)
				require.NoError(t, err)
				require.Empty(t, live)
			},
		},
		{
			name: "list since includes tombstone",
			run: func(t *testing.T) {
				since := time.Now().Add(-time.Hour)
				all, err := svc.List(t.Context(), owner, &since)
				require.NoError(t, err)
				require.Len(t, all, 1)
				require.NotNil(t, all[0].DeletedAt)
			},
		},
	}

	for _, tc := range steps {
		t.Run(tc.name, tc.run)
	}
}

func fillBytes(n int, fill byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return b
}
