package file_test

import (
	"testing"

	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/Radiushina/GophKeeper/pkg/tests/containers/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRepo_FileLifecycle(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("requires postgres container")
	}

	pool, _, _ := postgres.New(t)
	users := user.NewRepository(pool)
	files := file.NewRepository(pool)

	owner, err := users.CreateUser(t.Context(), testOwner("file-owner"))
	require.NoError(t, err)

	id := uuid.New()
	created, err := files.Create(t.Context(), file.File{
		ID:      id,
		UserID:  owner.ID,
		Version: 1,
		Nonce:   fillBytes(vault.NonceSize, 1),
		Meta:    "work",
	})
	require.NoError(t, err)
	require.Equal(t, "work", created.Meta)

	_, err = files.Create(t.Context(), file.File{
		ID: id, UserID: owner.ID, Version: 1, Nonce: fillBytes(vault.NonceSize, 1),
	})
	require.ErrorIs(t, err, file.ErrConflict)

	updated, err := files.Update(t.Context(), file.File{
		ID: id, UserID: owner.ID, Version: 1, Nonce: fillBytes(vault.NonceSize, 3), Meta: "home",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Version)

	deleted, err := files.SoftDelete(t.Context(), owner.ID, id)
	require.NoError(t, err)
	require.NotNil(t, deleted.DeletedAt)
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
