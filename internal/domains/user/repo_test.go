package user_test

import (
	"testing"

	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/Radiushina/GophKeeper/pkg/tests/containers/postgres"
	"github.com/stretchr/testify/require"
)

func testUser(login, pass string) user.User {
	p := vault.DefaultParams()
	m, err := vault.NewMaterial(pass, p)
	if err != nil {
		panic(err)
	}
	return user.User{
		Login:          login,
		Password:       pass,
		KdfSalt:        m.Salt,
		KdfMemory:      int(p.Memory),
		KdfIterations:  int(p.Iterations),
		KdfParallelism: int(p.Parallelism),
		KdfVersion:     int(p.Version),
		ProtectedKey:   m.ProtectedKey,
		KeyHash:        m.KeyHash,
	}
}

func TestRepo_Insert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		user    user.User
		prepare func(t *testing.T, repo *user.UsersRepo)
		wantErr error
	}{
		{
			name:    "Success insert user",
			user:    testUser("user1", "password"),
			wantErr: nil,
		},
		{
			name: "Not unique login",
			user: testUser("user1", "password2"),
			prepare: func(t *testing.T, repo *user.UsersRepo) {
				t.Helper()
				_, err := repo.CreateUser(t.Context(), testUser("user1", "password"))
				require.NoError(t, err)
			},
			wantErr: user.ErrUserAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := user.NewRepository(pool)

			if tc.prepare != nil {
				tc.prepare(t, repo)
			}

			created, err := repo.CreateUser(t.Context(), tc.user)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, created.ID)
			require.Equal(t, tc.user.Login, created.Login)
			require.Equal(t, tc.user.KdfSalt, created.KdfSalt)
			require.Equal(t, tc.user.KeyHash, created.KeyHash)
		})
	}
}

func TestRepo_SelectRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		login   string
		prepare func(t *testing.T, repo *user.UsersRepo) user.User
		wantErr error
	}{
		{
			name:  "Success get user",
			login: "user1",
			prepare: func(t *testing.T, repo *user.UsersRepo) user.User {
				t.Helper()
				created, err := repo.CreateUser(t.Context(), testUser("user1", "hashed-password"))
				require.NoError(t, err)
				return created
			},
			wantErr: nil,
		},
		{
			name:    "User not found",
			login:   "unknown",
			wantErr: user.ErrUserNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := user.NewRepository(pool)

			var want user.User
			if tc.prepare != nil {
				want = tc.prepare(t, repo)
			}

			found, err := repo.GetByLogin(t.Context(), tc.login)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want, found)
		})
	}
}
