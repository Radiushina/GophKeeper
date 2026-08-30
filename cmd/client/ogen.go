package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/client"
	"go.uber.org/zap"
)

func newCLI(cfg *config.Config, log *zap.Logger) (*client.App, error) {
	app := &client.App{Log: log}
	oasClient, err := oas.NewClient(cfg.Client.HTTP.Address, oas.WithClient(&http.Client{
		Transport: &authTransport{
			app:  app,
			base: &loggingTransport{log: log, base: http.DefaultTransport},
		},
	}))
	if err != nil {
		return nil, fmt.Errorf("oas client: %w", err)
	}
	app.Client = oasClient
	return app, nil
}

type loggingTransport struct {
	log  *zap.Logger
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	fields := []zap.Field{
		zap.String("uri", req.URL.RequestURI()),
		zap.String("method", req.Method),
		zap.Duration("duration", time.Since(start)),
	}
	if err != nil {
		t.log.Error("HTTP request", append(fields, zap.Error(err))...)
		return resp, err
	}
	t.log.Info("HTTP request",
		append(fields,
			zap.Int("status", resp.StatusCode),
			zap.Int64("response_size", resp.ContentLength),
		)...,
	)
	return resp, nil
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
