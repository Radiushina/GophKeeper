package client

import (
	"context"
	"fmt"

	"github.com/Radiushina/GophKeeper/gen/oas"
)

func Register(ctx context.Context, app *App, login, password string) error {
	res, err := app.Client.APIUserRegisterPost(ctx, &oas.APIUserRegisterPostReq{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}
	return handleAuthRes(app, res)
}

func Login(ctx context.Context, app *App, login, password string) error {
	res, err := app.Client.APIUserLoginPost(ctx, &oas.APIUserLoginPostReq{
		Login:    login,
		Password: password,
	})
	if err != nil {
		return err
	}
	return handleAuthRes(app, res)
}

func handleAuthRes(app *App, res any) error {
	switch v := res.(type) {
	case *oas.AuthUserResHeaders:
		rememberToken(app, v.Response.Token)
		app.logAuth(v)
		return nil
	case *oas.APIUserRegisterPostBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.APIUserRegisterPostConflict:
		return fmt.Errorf("%s", v.Msg)
	case *oas.APIUserRegisterPostInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	case *oas.APIUserLoginPostBadRequest:
		return fmt.Errorf("%s", v.Msg)
	case *oas.APIUserLoginPostUnauthorized:
		return fmt.Errorf("%s", v.Msg)
	case *oas.APIUserLoginPostInternalServerError:
		return fmt.Errorf("%s", v.Msg)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}
