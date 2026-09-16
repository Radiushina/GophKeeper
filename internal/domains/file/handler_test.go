package file_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_Files(t *testing.T) {
	t.Parallel()

	ts, token := startFileServer(t)
	id := uuid.New()

	body := mustJSON(&oas.CreateFile{
		ID: id, Version: 1, Nonce: fillBytes(vault.NonceSize, 9), Meta: oas.NewOptString("work"), Ciphertext: fillBytes(32, 8),
	})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/files", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	var got oas.File
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
	require.Equal(t, id, got.ID)
	require.Equal(t, oas.FileKindFile, got.Kind)
	require.Equal(t, "work", got.Meta.Value)
	require.NotEmpty(t, got.Ciphertext)

	listReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/files", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRes, err := http.DefaultClient.Do(listReq)
	require.NoError(t, err)
	defer listRes.Body.Close()
	require.Equal(t, http.StatusOK, listRes.StatusCode)
}

func startFileServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	tokens := user.NewJWT("file-secret", time.Hour)
	h := providers.NewOASHandler(
		user.NewHandler(user.NewService(newUserMem(), tokens, user.NewHasher()), nil),
		note.NewHandler(note.NewService(&noteMem{}), nil),
		card.NewMemHandler(),
		file.NewMemHandler(),
	)
	srv, err := oas.NewServer(h, tokens)
	require.NoError(t, err)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	token, err := tokens.Generate(uuid.New())
	require.NoError(t, err)
	return ts, token
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type userMem struct {
	byLogin map[string]user.User
}

func newUserMem() *userMem {
	return &userMem{byLogin: map[string]user.User{}}
}

func (m *userMem) CreateUser(_ context.Context, u user.User) (user.User, error) {
	return u, nil
}

func (m *userMem) GetByLogin(_ context.Context, _ string) (user.User, error) {
	return user.User{}, user.ErrUserNotFound
}

type noteMem struct{}

func (noteMem) Create(_ context.Context, n note.Note) (note.Note, error) { return n, nil }
func (noteMem) Update(_ context.Context, n note.Note) (note.Note, error) { return n, nil }
func (noteMem) SoftDelete(_ context.Context, _, _ uuid.UUID) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) Get(_ context.Context, _, _ uuid.UUID) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) List(_ context.Context, _ uuid.UUID, _ *time.Time) ([]note.Note, error) {
	return nil, nil
}
