package user_test

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

func TestHandler_Auth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(t *testing.T, base string, material vault.Material)
		method     string
		path       string
		body       func(material vault.Material) []byte
		wantStatus int
		check      func(t *testing.T, res *http.Response, material vault.Material)
	}{
		{
			name:       "register ok",
			method:     http.MethodPost,
			path:       "/api/v1/user/register",
			body:       registerBody("alice", "secret"),
			wantStatus: http.StatusOK,
			check: func(t *testing.T, res *http.Response, material vault.Material) {
				t.Helper()
				require.Contains(t, res.Header.Get("Authorization"), "Bearer ")
				var got oas.AuthUserRes
				require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
				require.Equal(t, "alice", got.User.Login)
				require.NotEmpty(t, got.Token)
				require.Equal(t, material.Salt, got.KdfSalt)
				require.Equal(t, material.ProtectedKey, got.ProtectedKey)
				require.Equal(t, material.KeyHash, got.KeyHash)
				require.Equal(t, vault.DefaultMemory, got.KdfParams.Memory)
			},
		},
		{
			name: "register duplicate",
			setup: func(t *testing.T, base string, material vault.Material) {
				t.Helper()
				postAuth(t, base+"/api/v1/user/register", registerBody("alice", "secret")(material), http.StatusOK)
			},
			method:     http.MethodPost,
			path:       "/api/v1/user/register",
			body:       registerBody("alice", "secret"),
			wantStatus: http.StatusConflict,
		},
		{
			name: "login wrong password",
			setup: func(t *testing.T, base string, material vault.Material) {
				t.Helper()
				postAuth(t, base+"/api/v1/user/register", registerBody("alice", "secret")(material), http.StatusOK)
			},
			method:     http.MethodPost,
			path:       "/api/v1/user/login",
			body:       loginBody("alice", "wrong"),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "login ok",
			setup: func(t *testing.T, base string, material vault.Material) {
				t.Helper()
				postAuth(t, base+"/api/v1/user/register", registerBody("alice", "secret")(material), http.StatusOK)
			},
			method:     http.MethodPost,
			path:       "/api/v1/user/login",
			body:       loginBody("alice", "secret"),
			wantStatus: http.StatusOK,
			check: func(t *testing.T, res *http.Response, material vault.Material) {
				t.Helper()
				var got oas.AuthUserRes
				require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
				require.Equal(t, material.KeyHash, got.KeyHash)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts, material := startUserServer(t)
			if tc.setup != nil {
				tc.setup(t, ts.URL, material)
			}

			req, err := http.NewRequest(tc.method, ts.URL+tc.path, bytes.NewReader(tc.body(material)))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()
			gotBody, _ := io.ReadAll(res.Body)
			require.Equal(t, tc.wantStatus, res.StatusCode, string(gotBody))
			res.Body = io.NopCloser(bytes.NewReader(gotBody))
			if tc.check != nil {
				tc.check(t, res, material)
			}
		})
	}
}

func startUserServer(t *testing.T) (*httptest.Server, vault.Material) {
	t.Helper()
	tokens := user.NewJWT("handler-secret", time.Hour)
	svc := user.NewService(newMemRepo(), tokens, user.NewHasher())
	h := providers.NewOASHandler(user.NewHandler(svc, nil), note.NewHandler(note.NewService(&noteMem{}), nil), card.NewMemHandler(), file.NewMemHandler())
	srv, err := oas.NewServer(h, tokens)
	require.NoError(t, err)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	material, err := vault.NewMaterial("secret", vault.DefaultParams())
	require.NoError(t, err)
	return ts, material
}

func registerBody(login, password string) func(vault.Material) []byte {
	return func(material vault.Material) []byte {
		body, err := json.Marshal(oas.RegisterReq{
			Login:        login,
			Password:     password,
			KdfSalt:      material.Salt,
			ProtectedKey: material.ProtectedKey,
			KeyHash:      material.KeyHash,
		})
		if err != nil {
			panic(err)
		}
		return body
	}
}

func loginBody(login, password string) func(vault.Material) []byte {
	return func(vault.Material) []byte {
		body, err := json.Marshal(map[string]string{"login": login, "password": password})
		if err != nil {
			panic(err)
		}
		return body
	}
}

func postAuth(t *testing.T, url string, body []byte, want int) {
	t.Helper()
	res, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	gotBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	require.Equal(t, want, res.StatusCode, string(gotBody))
}

type memRepo struct {
	byLogin map[string]user.User
}

func newMemRepo() *memRepo {
	return &memRepo{byLogin: map[string]user.User{}}
}

func (m *memRepo) CreateUser(_ context.Context, u user.User) (user.User, error) {
	if _, ok := m.byLogin[u.Login]; ok {
		return user.User{}, user.ErrUserAlreadyExists
	}
	u.ID = uuid.New()
	m.byLogin[u.Login] = u
	return u, nil
}

func (m *memRepo) GetByLogin(_ context.Context, login string) (user.User, error) {
	u, ok := m.byLogin[login]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}

type noteMem struct{}

func (noteMem) Create(context.Context, note.Note) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) Update(context.Context, note.Note) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) SoftDelete(context.Context, uuid.UUID, uuid.UUID) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) Get(context.Context, uuid.UUID, uuid.UUID) (note.Note, error) {
	return note.Note{}, note.ErrNotFound
}
func (noteMem) List(context.Context, uuid.UUID, *time.Time) ([]note.Note, error) {
	return nil, nil
}
