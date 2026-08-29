package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/oas"
	applogger "github.com/Radiushina/GophKeeper/internal/domains/logger"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/ogen-go/ogen/ogenerrors"
	"go.uber.org/zap"
)

func NewHTTPServer(cfg *config.Config, h *user.Handler, jwt *user.JWT, log *zap.Logger) (*http.Server, error) {
	handler, err := oas.NewServer(h, oas.WithErrorHandler(OASErrorHandler))
	if err != nil {
		return nil, fmt.Errorf("oas server: %w", err)
	}

	return &http.Server{
		Addr:              cfg.Server.HTTP.Address,
		Handler:           applogger.LoggingMiddleware(log, user.NewAuthMiddleware(jwt)(handler)),
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}

func OASErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	code := ogenerrors.ErrorCode(err)
	msg := "internal server error"
	switch code {
	case http.StatusUnauthorized:
		msg = "user unauthorized"
	case http.StatusBadRequest:
		msg = "validate"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"msg": msg})
}
