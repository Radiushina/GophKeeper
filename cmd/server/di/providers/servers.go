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
	grpc *GRPCListen
	log  *zap.Logger
}

func NewServers(httpServer *http.Server, grpcSrv *GRPCListen, log *zap.Logger) *Servers {
	return &Servers{http: httpServer, grpc: grpcSrv, log: log}
}

func (s *Servers) Start(ctx context.Context) error {
	errCh := make(chan error, 2)
	go func() {
		s.log.Info("starting HTTP server", zap.String("addr", s.http.Addr))
		err := s.http.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	if s.grpc != nil && s.grpc.Server != nil {
		go func() {
			ln, err := s.grpc.listen()
			if err != nil {
				errCh <- err
				return
			}
			s.log.Info("starting gRPC server", zap.String("addr", s.grpc.Addr))
			err = s.grpc.Server.Serve(ln)
			if err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}

	select {
	case err := <-errCh:
		if err != nil {
			s.log.Error("listen", zap.Error(err))
			return err
		}
		return nil
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutCtx); err != nil {
			s.log.Error("http shutdown", zap.Error(err))
			return err
		}
		if s.grpc != nil && s.grpc.Server != nil {
			s.grpc.Server.GracefulStop()
		}
		s.log.Info("HTTP server stopped")
		<-errCh
		return nil
	}
}
