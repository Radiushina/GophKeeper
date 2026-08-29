package user

import (
	"context"
	"errors"

	"github.com/Radiushina/GophKeeper/gen/oas"
)

type (
	Handler struct {
		service ServiceProvider
	}

	ServiceProvider interface {
		CreateUser(ctx context.Context, login, password string) (AuthUserResponse, error)
		GetByLogin(ctx context.Context, login, password string) (AuthUserResponse, error)
	}
)

func NewHandler(service ServiceProvider) *Handler {
	return &Handler{service: service}
}

func (h *Handler) APIUserRegisterPost(ctx context.Context, req *oas.APIUserRegisterPostReq) (oas.APIUserRegisterPostRes, error) {
	if req.GetLogin() == "" || req.GetPassword() == "" {
		return &oas.APIUserRegisterPostBadRequest{Msg: "validate"}, nil
	}

	session, err := h.service.CreateUser(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			return &oas.APIUserRegisterPostConflict{Msg: "login is already taken"}, nil
		}
		if errors.Is(err, ErrInvalidCredentials) {
			return &oas.APIUserRegisterPostBadRequest{Msg: "validate"}, nil
		}
		return &oas.APIUserRegisterPostInternalServerError{Msg: "internal server error"}, nil
	}
	return authHeaders(session), nil
}

func (h *Handler) APIUserLoginPost(ctx context.Context, req *oas.APIUserLoginPostReq) (oas.APIUserLoginPostRes, error) {
	if req.GetLogin() == "" || req.GetPassword() == "" {
		return &oas.APIUserLoginPostBadRequest{Msg: "validate"}, nil
	}

	session, err := h.service.GetByLogin(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrUserNotFound) {
			return &oas.APIUserLoginPostUnauthorized{Msg: "invalid login/password pair"}, nil
		}
		return &oas.APIUserLoginPostInternalServerError{Msg: "internal server error"}, nil
	}
	return authHeaders(session), nil
}

func authHeaders(session AuthUserResponse) *oas.AuthUserResHeaders {
	return &oas.AuthUserResHeaders{
		Authorization: oas.NewOptString("Bearer " + session.Token),
		Response: oas.AuthUserRes{
			User: oas.User{
				ID:    session.User.ID,
				Login: session.User.Login,
			},
			Token: session.Token,
		},
	}
}
