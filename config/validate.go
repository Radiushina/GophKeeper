package config

import (
	"errors"
	"strings"
)

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server.HTTP.Address) == "" {
		return errors.New("server.http.address is required")
	}
	if strings.TrimSpace(c.Server.GRPC.Address) == "" {
		return errors.New("server.grpc.address is required")
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
	if strings.TrimSpace(c.S3.Endpoint) == "" {
		return errors.New("s3.endpoint is required")
	}
	if strings.TrimSpace(c.S3.Bucket) == "" {
		return errors.New("s3.bucket is required")
	}
	if strings.TrimSpace(c.S3.AccessKey) == "" {
		return errors.New("s3.access_key is required")
	}
	if strings.TrimSpace(c.S3.SecretKey) == "" {
		return errors.New("s3.secret_key is required")
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
