package client

import (
	"context"
	"fmt"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/vault"
)

func Register(ctx context.Context, app *App, login, password string) error {
	material, err := vault.NewMaterial(password, vault.DefaultParams())
	if err != nil {
		return fmt.Errorf("vault material: %w", err)
	}

	res, err := app.Client.AuthRegister(ctx, &oas.RegisterReq{
		Login:        login,
		Password:     password,
		KdfSalt:      material.Salt,
		ProtectedKey: material.ProtectedKey,
		KeyHash:      material.KeyHash,
	})
	if err != nil {
		return err
	}
	return handleAuthRes(app, res, password)
}

func Login(ctx context.Context, app *App, login, password string) error {
	res, err := app.Client.AuthLogin(ctx, &oas.AuthLoginReq{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}
	return handleAuthRes(app, res, password)
}

func handleAuthRes(app *App, res any, password string) error {
	switch v := res.(type) {
	case *oas.AuthUserResHeaders:
		rememberToken(app, v.Response.Token)
		app.SetUser(v.Response.User.Login)
		if err := unlockVault(app, password, v.Response); err != nil {
			return err
		}
		app.logAuth(v)
		return nil
	case *oas.AuthRegisterBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.AuthRegisterConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.AuthRegisterInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.AuthLoginBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.AuthLoginUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.AuthLoginInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

func unlockVault(app *App, password string, session oas.AuthUserRes) error {
	p := vault.Params{
		Memory:      uint32(session.KdfParams.Memory),
		Iterations:  uint32(session.KdfParams.Iterations),
		Parallelism: uint8(session.KdfParams.Parallelism),
		Version:     uint8(session.KdfParams.Version),
	}
	vk, err := vault.Unwrap(password, vault.Material{
		Salt:         session.KdfSalt,
		ProtectedKey: session.ProtectedKey,
		KeyHash:      session.KeyHash,
	}, p)
	if err != nil {
		return fmt.Errorf("unlock vault: %w", err)
	}
	app.SetVaultKey(vk)
	return nil
}
