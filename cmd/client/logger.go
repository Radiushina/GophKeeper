package main

import (
	"fmt"

	"github.com/Radiushina/GophKeeper/config"
	applogger "github.com/Radiushina/GophKeeper/internal/domains/logger"
	"go.uber.org/zap"
)

func newLogger(cfg *config.Config) (*zap.Logger, func(), error) {
	zl, err := applogger.New(cfg.Log.Level)
	if err != nil {
		return nil, nil, fmt.Errorf("logger: %w", err)
	}
	return zl, func() { _ = zl.Sync() }, nil
}
