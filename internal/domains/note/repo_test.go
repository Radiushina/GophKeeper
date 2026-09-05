package note_test

import (
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/Radiushina/GophKeeper/pkg/tests/containers/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRepo_NoteLifecycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("requires postgres container")
	}

	pool, _, _ := postgres.New(t)
	users := user.NewRepository(pool)
	notes := note.NewRepository(pool)

	owner, err := users.CreateUser(t.Context(), testOwner("note-owner"))
	require.NoError(t, err)

	id := uuid.New()
	var deleted note.Note

	steps := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create",
			run: func(t *testing.T) {
				created, err := notes.Create(t.Context(), note.Note{
					ID:         id,
					UserID:     owner.ID,
					Version:    1,
					Nonce:      fillBytes(vault.NonceSize, 1),
					Ciphertext: fillBytes(32, 2),
				})
				require.NoError(t, err)
				require.Equal(t, id, created.ID)
			},
		},
		{
			name: "create duplicate",
			run: func(t *testing.T) {
				_, err := notes.Create(t.Context(), note.Note{
					ID: id, UserID: owner.ID, Version: 1, Nonce: fillBytes(vault.NonceSize, 1), Ciphertext: fillBytes(32, 2),
				})
				require.ErrorIs(t, err, note.ErrConflict)
			},
		},
		{
			name: "update",
			run: func(t *testing.T) {
				updated, err := notes.Update(t.Context(), note.Note{
					ID: id, UserID: owner.ID, Version: 1, Nonce: fillBytes(vault.NonceSize, 3), Ciphertext: fillBytes(32, 4),
				})
				require.NoError(t, err)
				require.Equal(t, int64(2), updated.Version)
			},
		},
		{
			name: "stale update",
			run: func(t *testing.T) {
				_, err := notes.Update(t.Context(), note.Note{
					ID: id, UserID: owner.ID, Version: 1, Nonce: fillBytes(vault.NonceSize, 3), Ciphertext: fillBytes(32, 4),
				})
				require.ErrorIs(t, err, note.ErrConflict)
			},
		},
		{
			name: "list live",
			run: func(t *testing.T) {
				live, err := notes.List(t.Context(), owner.ID, nil)
				require.NoError(t, err)
				require.Len(t, live, 1)
			},
		},
		{
			name: "soft delete",
			run: func(t *testing.T) {
				var err error
				deleted, err = notes.SoftDelete(t.Context(), owner.ID, id)
				require.NoError(t, err)
				require.NotNil(t, deleted.DeletedAt)
			},
		},
		{
			name: "soft delete idempotent",
			run: func(t *testing.T) {
				again, err := notes.SoftDelete(t.Context(), owner.ID, id)
				require.NoError(t, err)
				require.Equal(t, deleted.Version, again.Version)
			},
		},
		{
			name: "list live empty",
			run: func(t *testing.T) {
				empty, err := notes.List(t.Context(), owner.ID, nil)
				require.NoError(t, err)
				require.Empty(t, empty)
			},
		},
		{
			name: "list since includes tombstone",
			run: func(t *testing.T) {
				since := time.Now().Add(-time.Hour)
				withTomb, err := notes.List(t.Context(), owner.ID, &since)
				require.NoError(t, err)
				require.Len(t, withTomb, 1)
			},
		},
		{
			name: "get missing",
			run: func(t *testing.T) {
				_, err := notes.Get(t.Context(), owner.ID, uuid.New())
				require.ErrorIs(t, err, note.ErrNotFound)
			},
		},
	}

	for _, tc := range steps {
		t.Run(tc.name, tc.run)
	}
}

func testOwner(login string) user.User {
	p := vault.DefaultParams()
	m, err := vault.NewMaterial("secret", p)
	if err != nil {
		panic(err)
	}
	return user.User{
		Login:          login,
		Password:       "hash",
		KdfSalt:        m.Salt,
		KdfMemory:      int(p.Memory),
		KdfIterations:  int(p.Iterations),
		KdfParallelism: int(p.Parallelism),
		KdfVersion:     int(p.Version),
		ProtectedKey:   m.ProtectedKey,
		KeyHash:        m.KeyHash,
	}
}
