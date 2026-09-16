package file_test

import (
	"crypto/sha256"
	"testing"

	"github.com/Radiushina/GophKeeper/internal/blob"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService_CreateGetListDelete(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	id := uuid.New()
	nonce := fillBytes(vault.NonceSize, 1)
	ct := fillBytes(32, 2)
	sum := sha256.Sum256(ct)
	svc := file.NewService(file.NewMemRepo(), blob.NewMem())

	created, err := svc.Create(t.Context(), owner, file.CreateInput{
		ID: id, Version: file.CreateVersion, Nonce: nonce, Meta: "work", Ciphertext: ct, CiphertextSHA256: sum[:],
	})
	require.NoError(t, err)
	require.Equal(t, ct, created.Ciphertext)

	got, err := svc.Get(t.Context(), owner, id)
	require.NoError(t, err)
	require.Equal(t, ct, got.Ciphertext)
	require.Equal(t, "work", got.Meta)

	list, err := svc.List(t.Context(), owner, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)

	_, err = svc.Create(t.Context(), owner, file.CreateInput{
		ID: id, Version: 1, Nonce: nonce, Ciphertext: ct,
	})
	require.ErrorIs(t, err, file.ErrConflict)

	_, err = svc.Create(t.Context(), owner, file.CreateInput{
		ID: uuid.New(), Version: 2, Nonce: nonce, Ciphertext: ct,
	})
	require.ErrorIs(t, err, file.ErrConflict)

	tomb, err := svc.Delete(t.Context(), owner, id)
	require.NoError(t, err)
	require.NotNil(t, tomb.DeletedAt)

	_, err = svc.Get(t.Context(), owner, id)
	require.NoError(t, err)
}

func TestService_ChunksResume(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	id := uuid.New()
	svc := file.NewService(file.NewMemRepo(), blob.NewMem())
	nonce := fillBytes(vault.NonceSize, 3)
	ct := fillBytes(32, 4)

	require.NoError(t, svc.PutChunk(t.Context(), owner, id, 0, nonce, ct))
	require.NoError(t, svc.PutChunk(t.Context(), owner, id, 1, nonce, ct))
	st, err := svc.UploadStatus(t.Context(), owner, id)
	require.NoError(t, err)
	require.Equal(t, int32(2), st.NextIndex)

	out, err := svc.FinishUpload(t.Context(), owner, file.UploadHeader{
		ID: id, Version: file.CreateVersion, Name: "a.bin", Meta: "tag", Size: 10,
	}, 2)
	require.NoError(t, err)
	require.Equal(t, int32(2), out.ChunkCount)
	require.Equal(t, "a.bin", out.Name)

	got, err := svc.Get(t.Context(), owner, id)
	require.NoError(t, err)
	require.Empty(t, got.Ciphertext)
	require.Equal(t, int32(2), got.ChunkCount)

	_, err = svc.FinishUpload(t.Context(), owner, file.UploadHeader{
		ID: uuid.New(), Version: 1, Size: file.MaxFileSize + 1,
	}, 1)
	require.ErrorIs(t, err, file.ErrTooLarge)
}

func fillBytes(n int, fill byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return b
}
