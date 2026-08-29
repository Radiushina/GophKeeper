package providers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Servers struct {
	http *http.Server
	log  *zap.Logger
}

func NewServers(httpServer *http.Server, log *zap.Logger) *Servers {
	return &Servers{http: httpServer, log: log}
}

func (s *Servers) Start(ctx context.Context) {
	go func() {
		s.log.Info("starting HTTP server", zap.String("addr", s.http.Addr))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("http listen", zap.Error(err))
		}
	}()

	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.http.Shutdown(shutCtx); err != nil {
		s.log.Error("http shutdown", zap.Error(err))
		return
	}
	s.log.Info("HTTP server stopped")
}
