package config

import (
	"errors"
	"strings"
)

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server.HTTP.Address) == "" {
		return errors.New("server.http.address is required")
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return errors.New("database.dsn is required")
	}
	if strings.TrimSpace(c.Auth.JWTSecret) == "" {
		return errors.New("auth.jwt_secret is required")
	}
	if c.Auth.JWTTTL <= 0 {
		return errors.New("auth.jwt_ttl must be positive")
	}
	if strings.TrimSpace(c.Log.Level) == "" {
		return errors.New("log.level is required")
	}
	return nil
}

func (c *Config) ValidateClient() error {
	if strings.TrimSpace(c.Client.HTTP.Address) == "" {
		return errors.New("client.http.address is required")
	}
	if strings.TrimSpace(c.Log.Level) == "" {
		return errors.New("log.level is required")
	}
	return nil
}
