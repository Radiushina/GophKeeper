package client

import (
	"io"
	"strings"
	"sync"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"go.uber.org/zap"
)

type App struct {
	mu     sync.Mutex
	token  string
	user   string
	Server string
	Client *oas.Client
	Log    *zap.Logger
}

func (a *App) logError(msg string, err error, fields ...zap.Field) {
	if a == nil || a.Log == nil || err == nil {
		return
	}
	a.Log.Warn(msg, append(fields, zap.Error(err))...)
}

func (a *App) logInfo(msg string, fields ...zap.Field) {
	if a == nil || a.Log == nil {
		return
	}
	a.Log.Info(msg, fields...)
}

func (a *App) logWriter() io.Writer {
	if a == nil || a.Log == nil {
		return io.Discard
	}
	return zapWriter{log: a.Log}
}

type zapWriter struct {
	log *zap.Logger
}

func (w zapWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.log.Warn(msg)
	}
	return len(p), nil
}

func (a *App) SetToken(token string) {
	a.mu.Lock()
	a.token = token
	a.mu.Unlock()
}

func (a *App) Token() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}

func (a *App) SetUser(login string) {
	a.mu.Lock()
	a.user = login
	a.mu.Unlock()
}

func (a *App) User() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.user
}

func rememberToken(a *App, token string) {
	a.SetToken(token)
}

func (a *App) logAuth(session *oas.AuthUserResHeaders) {
	a.logInfo("authenticated",
		zap.String("user", session.Response.User.Login),
		zap.String("token", session.Response.Token),
	)
}
