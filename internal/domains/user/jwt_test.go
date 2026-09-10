package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJWT_Parse(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	valid := user.NewJWT("test-secret", time.Hour)
	raw, err := valid.Generate(id)
	require.NoError(t, err)

	expired := user.NewJWT("test-secret", time.Nanosecond)
	expiredRaw, err := expired.Generate(uuid.New())
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	tests := []struct {
		name    string
		tokens  *user.JWT
		raw     string
		wantID  uuid.UUID
		wantErr error
	}{
		{
			name:   "valid",
			tokens: valid,
			raw:    raw,
			wantID: id,
		},
		{
			name:    "garbage",
			tokens:  valid,
			raw:     "not-a-jwt",
			wantErr: user.ErrUnauthorized,
		},
		{
			name:    "wrong secret",
			tokens:  user.NewJWT("other-secret", time.Hour),
			raw:     raw,
			wantErr: user.ErrUnauthorized,
		},
		{
			name:    "expired",
			tokens:  expired,
			raw:     expiredRaw,
			wantErr: user.ErrUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.tokens.Parse(tc.raw)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantID, got)
		})
	}
}

func TestJWT_HandleBearerAuth(t *testing.T) {
	t.Parallel()

	j := user.NewJWT("test-secret", time.Hour)
	id := uuid.New()

	ctx := ctxWithUserID(t, j, id)
	got, err := user.IDFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, id, got)

	_, err = j.HandleBearerAuth(context.Background(), "", oas.BearerAuth{})
	require.Error(t, err)
}

// ctxWithUserID builds an authenticated ctx the same way production does (via HandleBearerAuth),
// without exporting user.withUserID.
func ctxWithUserID(t *testing.T, j *user.JWT, id uuid.UUID) context.Context {
	t.Helper()
	tok, err := j.Generate(id)
	require.NoError(t, err)
	ctx, err := j.HandleBearerAuth(context.Background(), "", oas.BearerAuth{Token: tok})
	require.NoError(t, err)
	return ctx
}
