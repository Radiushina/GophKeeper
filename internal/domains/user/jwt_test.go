package user_test

import (
	"testing"
	"time"

	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJWT_GenerateAndParse(t *testing.T) {
	t.Parallel()

	tokens := user.NewJWT("test-secret", time.Hour)
	id := uuid.New()

	raw, err := tokens.Generate(id)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	got, err := tokens.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, id, got)
}

func TestJWT_ParseRejectsGarbageAndWrongSecret(t *testing.T) {
	t.Parallel()

	tokens := user.NewJWT("test-secret", time.Hour)
	_, err := tokens.Parse("not-a-jwt")
	require.ErrorIs(t, err, user.ErrUnauthorized)

	other := user.NewJWT("other-secret", time.Hour)
	id := uuid.New()
	raw, err := tokens.Generate(id)
	require.NoError(t, err)

	_, err = other.Parse(raw)
	require.ErrorIs(t, err, user.ErrUnauthorized)
}

func TestJWT_ParseRejectsExpired(t *testing.T) {
	t.Parallel()

	tokens := user.NewJWT("test-secret", time.Nanosecond)
	raw, err := tokens.Generate(uuid.New())
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	_, err = tokens.Parse(raw)
	require.ErrorIs(t, err, user.ErrUnauthorized)
}
