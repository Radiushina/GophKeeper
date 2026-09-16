package main

import (
	"fmt"
	"strings"

	"github.com/Radiushina/GophKeeper/config"
)

func newConfig(f *Flags) (*config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.Client.HTTP.Address = f.serverBaseURL()
	cfg.Log.Level = strings.TrimSpace(f.logLevel)
	if err := cfg.ValidateClient(); err != nil {
		return nil, fmt.Errorf("configuration validation: %w", err)
	}
	return &cfg, nil
}
