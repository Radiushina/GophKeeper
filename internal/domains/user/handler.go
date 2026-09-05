package user

import (
	"context"
	"errors"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/vault"
	"go.uber.org/zap"
)

type (
	Handler struct {
		oas.UserHandler

		service ServiceProvider
		log     *zap.Logger
	}

	ServiceProvider interface {
		CreateUser(ctx context.Context, in RegisterInput) (AuthUserResponse, error)
		GetByLogin(ctx context.Context, login, password string) (AuthUserResponse, error)
	}
)

func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{service: service, log: log}
}

func (h *Handler) AuthRegister(ctx context.Context, req *oas.RegisterReq) (oas.AuthRegisterRes, error) {
	session, err := h.service.CreateUser(ctx, RegisterInput{
		Login:        req.GetLogin(),
		Password:     req.GetPassword(),
		KdfSalt:      req.GetKdfSalt(),
		ProtectedKey: req.GetProtectedKey(),
		KeyHash:      req.GetKeyHash(),
	})
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			return &oas.AuthRegisterConflict{Msg: "login is already taken"}, nil
		}
		if errors.Is(err, ErrInvalidCredentials) {
			return &oas.AuthRegisterBadRequest{Msg: "validate"}, nil
		}
		h.log.Error("register", zap.Error(err))
		return &oas.AuthRegisterInternalServerError{Msg: "internal server error"}, nil
	}
	return authHeaders(session), nil
}

func (h *Handler) AuthLogin(ctx context.Context, req *oas.AuthLoginReq) (oas.AuthLoginRes, error) {
	if req.GetLogin() == "" || req.GetPassword() == "" {
		return &oas.AuthLoginBadRequest{Msg: "validate"}, nil
	}

	session, err := h.service.GetByLogin(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrUserNotFound) {
			return &oas.AuthLoginUnauthorized{Msg: "invalid login/password pair"}, nil
		}
		h.log.Error("login", zap.Error(err))
		return &oas.AuthLoginInternalServerError{Msg: "internal server error"}, nil
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
			Token:        session.Token,
			KdfSalt:      session.KdfSalt,
			KdfParams:    toOASKdfParams(session.KdfParams),
			ProtectedKey: session.ProtectedKey,
			KeyHash:      session.KeyHash,
		},
	}
}

func toOASKdfParams(p vault.Params) oas.KdfParams {
	return oas.KdfParams{
		Algorithm:   oas.KdfParamsAlgorithmArgon2id,
		Memory:      int(p.Memory),
		Iterations:  int(p.Iterations),
		Parallelism: int(p.Parallelism),
		Version:     oas.KdfParamsVersion(p.Version),
	}
}
