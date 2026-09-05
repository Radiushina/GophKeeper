package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/oas"
	applogger "github.com/Radiushina/GophKeeper/internal/domains/logger"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/ogen-go/ogen/validate"
	"go.uber.org/zap"
)

func init() {
	_ = validate.RegisterValidator("notBlank", func(value, _ any) error {
		s, ok := value.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return fmt.Errorf("must not be blank")
		}
		return nil
	})
}

func NewHTTPServer(cfg *config.Config, h oas.Handler, jwt *user.JWT, log *zap.Logger) (*http.Server, error) {
	handler, err := oas.NewServer(h, jwt, oas.WithErrorHandler(OASErrorHandler))
	if err != nil {
		return nil, fmt.Errorf("oas server: %w", err)
	}

	return &http.Server{
		Addr:              cfg.Server.HTTP.Address,
		Handler:           applogger.LoggingMiddleware(log, handler),
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
