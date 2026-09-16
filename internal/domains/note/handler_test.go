package note_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestHandler_Notes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       func(id uuid.UUID) string
		auth       bool
		body       func(id uuid.UUID) []byte
		setup      func(t *testing.T, base, token string, id uuid.UUID)
		wantStatus int
		check      func(t *testing.T, res *http.Response, id uuid.UUID)
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   func(uuid.UUID) string { return "/api/v1/notes" },
			auth:   true,
			body: func(id uuid.UUID) []byte {
				return mustJSON(&oas.CreateNote{
					ID: id, Version: 1, Nonce: fillBytes(vault.NonceSize, 9), Meta: oas.NewOptString("work"), Ciphertext: fillBytes(32, 8),
				})
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, res *http.Response, id uuid.UUID) {
				t.Helper()
				var got oas.Note
				require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
				require.Equal(t, id, got.ID)
				require.Equal(t, int64(1), got.Version)
				require.Equal(t, oas.NoteKindNote, got.Kind)
				require.Equal(t, "work", got.Meta.Value)
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   func(id uuid.UUID) string { return "/api/v1/notes/" + id.String() },
			auth:   true,
			setup: func(t *testing.T, base, token string, id uuid.UUID) {
				t.Helper()
				createNote(t, base, token, id)
			},
			body: func(uuid.UUID) []byte {
				return mustJSON(&oas.UpdateNote{Version: 1, Nonce: fillBytes(vault.NonceSize, 7), Ciphertext: fillBytes(32, 6)})
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "stale version",
			method: http.MethodPut,
			path:   func(id uuid.UUID) string { return "/api/v1/notes/" + id.String() },
			auth:   true,
			setup: func(t *testing.T, base, token string, id uuid.UUID) {
				t.Helper()
				createNote(t, base, token, id)
				doJSON(t, http.MethodPut, base+"/api/v1/notes/"+id.String(), token, mustJSON(&oas.UpdateNote{
					Version: 1, Nonce: fillBytes(vault.NonceSize, 7), Ciphertext: fillBytes(32, 6),
				}), http.StatusOK)
			},
			body: func(uuid.UUID) []byte {
				return mustJSON(&oas.UpdateNote{Version: 1, Nonce: fillBytes(vault.NonceSize, 7), Ciphertext: fillBytes(32, 6)})
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   func(id uuid.UUID) string { return "/api/v1/notes/" + id.String() },
			auth:   true,
			setup: func(t *testing.T, base, token string, id uuid.UUID) {
				t.Helper()
				createNote(t, base, token, id)
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, res *http.Response, _ uuid.UUID) {
				t.Helper()
				var tomb oas.Note
				require.NoError(t, json.NewDecoder(res.Body).Decode(&tomb))
				require.True(t, tomb.DeletedAt.IsSet())
			},
		},
		{
			name:       "list without auth",
			method:     http.MethodGet,
			path:       func(uuid.UUID) string { return "/api/v1/notes" },
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts, token := startNoteServer(t)
			id := uuid.New()
			if tc.setup != nil {
				tc.setup(t, ts.URL, token, id)
			}

			var body io.Reader
			if tc.body != nil {
				body = bytes.NewReader(tc.body(id))
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path(id), body)
			require.NoError(t, err)
			if tc.auth {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			if tc.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.wantStatus, res.StatusCode)
			if tc.check != nil {
				tc.check(t, res, id)
			}
		})
	}
}

func startNoteServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	tokens := user.NewJWT("note-secret", time.Hour)
	h := providers.NewOASHandler(
		user.NewHandler(user.NewService(newUserMem(), tokens, user.NewHasher()), nil),
		note.NewHandler(note.NewService(newMemNotes()), nil),
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

func createNote(t *testing.T, base, token string, id uuid.UUID) {
	t.Helper()
	doJSON(t, http.MethodPost, base+"/api/v1/notes", token, mustJSON(&oas.CreateNote{
		ID: id, Version: 1, Nonce: fillBytes(vault.NonceSize, 9), Ciphertext: fillBytes(32, 8),
	}), http.StatusOK)
}

func doJSON(t *testing.T, method, url, token string, body []byte, want int) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	require.Equal(t, want, res.StatusCode)
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
