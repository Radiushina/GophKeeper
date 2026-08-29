package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandler_RegisterLogin(t *testing.T) {
	t.Parallel()

	tokens := user.NewJWT("handler-secret", time.Hour)
	svc := user.NewService(newMemRepo(), tokens, user.NewHasher())
	h := user.NewHandler(svc)
	srv, err := oas.NewServer(h)
	require.NoError(t, err)

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	body, _ := json.Marshal(map[string]string{"login": "alice", "password": "secret"})
	res, err := http.Post(ts.URL+"/api/user/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Contains(t, res.Header.Get("Authorization"), "Bearer ")

	var registered struct {
		User  struct{ Login string }
		Token string
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&registered))
	require.Equal(t, "alice", registered.User.Login)
	require.NotEmpty(t, registered.Token)

	dup, err := http.Post(ts.URL+"/api/user/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	dup.Body.Close()
	require.Equal(t, http.StatusConflict, dup.StatusCode)

	loginBody, _ := json.Marshal(map[string]string{"login": "alice", "password": "wrong"})
	bad, err := http.Post(ts.URL+"/api/user/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	bad.Body.Close()
	require.Equal(t, http.StatusUnauthorized, bad.StatusCode)

	okLogin, err := http.Post(ts.URL+"/api/user/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer okLogin.Body.Close()
	require.Equal(t, http.StatusOK, okLogin.StatusCode)
}

type memRepo struct {
	byLogin map[string]user.User
}

func newMemRepo() *memRepo {
	return &memRepo{byLogin: map[string]user.User{}}
}

func (m *memRepo) CreateUser(_ context.Context, login, hashedPassword string) (user.User, error) {
	if _, ok := m.byLogin[login]; ok {
		return user.User{}, user.ErrUserAlreadyExists
	}
	u := user.User{ID: uuid.New(), Login: login, Password: hashedPassword}
	m.byLogin[login] = u
	return u, nil
}

func (m *memRepo) GetByLogin(_ context.Context, login string) (user.User, error) {
	u, ok := m.byLogin[login]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}
