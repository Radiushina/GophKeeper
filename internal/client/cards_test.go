package client

import (
	"testing"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMaskCardNumber(t *testing.T) {
	t.Parallel()
	require.Equal(t, "****4242", maskCardNumber("4111111111114242"))
	require.Equal(t, "1234", maskCardNumber("1234"))
}

func TestSealOpenCard(t *testing.T) {
	t.Parallel()
	app := &App{}
	vk := make([]byte, vault.VKSize)
	for i := range vk {
		vk[i] = byte(i + 1)
	}
	app.SetVaultKey(vk)

	plain := CardPlain{Number: "4111111111114242", Holder: "ADA", Expiry: "12/30", CVV: "123"}
	blob, err := sealCard(app, plain)
	require.NoError(t, err)

	got, meta, err := openCard(app, oas.Card{
		ID: uuid.New(), Version: 1, Nonce: blob.nonce, Ciphertext: blob.ciphertext, Meta: oas.NewOptString("visa"),
	})
	require.NoError(t, err)
	require.Equal(t, plain, got)
	require.Equal(t, "visa", meta)
}

func TestParseCardFlags(t *testing.T) {
	t.Parallel()
	app := &App{}

	plain, meta, err := parseCardAddFlags(app, []string{
		"-number", "4111", "-holder", "ADA", "-expiry", "12/30", "-cvv", "123", "-meta", "visa",
	})
	require.NoError(t, err)
	require.Equal(t, "4111", plain.Number)
	require.Equal(t, "visa", meta)

	_, _, err = parseCardAddFlags(app, []string{"-holder", "ADA"})
	require.Error(t, err)

	upd, err := parseCardUpdateFlags(app, []string{
		"-id", uuid.New().String(), "-number", "4111", "-holder", "ADA", "-expiry", "12/30", "-cvv", "123",
	})
	require.NoError(t, err)
	require.Equal(t, "4111", upd.plain.Number)

	_, err = parseCardUpdateFlags(app, []string{"-id", uuid.New().String()})
	require.Error(t, err)
}
