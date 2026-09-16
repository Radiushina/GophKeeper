package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/filepb"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/client"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func newCLI(cfg *config.Config, log *zap.Logger) (*client.App, error) {
	cacheDir := filepath.Join(os.TempDir(), "gophkeeper")
	if dir, err := os.UserConfigDir(); err == nil {
		cacheDir = filepath.Join(dir, "gophkeeper")
	}
	app := &client.App{Log: log, Server: cfg.Client.HTTP.Address, CacheDir: cacheDir}
	sec := tokenSource{app: app}
	oasClient, err := oas.NewClient(cfg.Client.HTTP.Address, sec, oas.WithClient(&http.Client{
		Timeout: 15 * time.Second,
		Transport: &authTransport{
			app:  app,
			base: &loggingTransport{app: app, base: http.DefaultTransport},
		},
	}))
	if err != nil {
		return nil, fmt.Errorf("oas client: %w", err)
	}
	app.Client = oasClient
	if cfg.Client.GRPC.Address != "" {
		creds, err := loadClientTLS(cfg.Client.GRPC.TLS.CAFile, cfg.Client.GRPC.TLS.ServerName)
		if err != nil {
			return nil, fmt.Errorf("grpc tls: %w", err)
		}
		conn, err := grpc.NewClient(
			cfg.Client.GRPC.Address,
			grpc.WithTransportCredentials(creds),
		)
		if err != nil {
			return nil, fmt.Errorf("grpc files: %w", err)
		}
		app.Files = filepb.NewFileServiceClient(conn)
	}
	return app, nil
}

type tokenSource struct {
	app *client.App
}

func (s tokenSource) BearerAuth(_ context.Context, _ oas.OperationName) (oas.BearerAuth, error) {
	return oas.BearerAuth{Token: s.app.Token()}, nil
}

type loggingTransport struct {
	app  *client.App
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	log := (*zap.Logger)(nil)
	if t.app != nil {
		log = t.app.Log
	}
	if log == nil {
		return resp, err
	}
	fields := []zap.Field{
		zap.String("uri", req.URL.RequestURI()),
		zap.String("method", req.Method),
		zap.Duration("duration", time.Since(start)),
	}
	if err != nil {
		log.Error("HTTP request", append(fields, zap.Error(err))...)
		return resp, err
	}
	log.Info("HTTP request",
		append(fields,
			zap.Int("status", resp.StatusCode),
			zap.Int64("response_size", resp.ContentLength),
		)...,
	)
	return resp, err
}

type authTransport struct {
	app  *client.App
	base http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if token := t.app.Token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return t.base.RoundTrip(req)
}

func loadClientTLS(caFile, serverName string) (credentials.TransportCredentials, error) {
	// Читаем ca.crt — «справочник печатей, которым мы доверяем».
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read ca: %w", err)
	}

	// CertPool — набор доверенных CA.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("append ca pem from %s", caFile)
	}

	tlsCfg := &tls.Config{
		RootCAs:    pool,       // проверяем server.crt этой CA
		ServerName: serverName, // ожидаемое имя в сертификате (localhost)
		MinVersion: tls.VersionTLS12,
	}
	return credentials.NewTLS(tlsCfg), nil
}
