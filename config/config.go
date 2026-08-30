package config

import "time"

type Config struct {
	Server   ServerConfig   `koanf:"server" yaml:"server"`
	Client   ClientConfig   `koanf:"client" yaml:"client"`
	Database DatabaseConfig `koanf:"database" yaml:"database"`
	Auth     AuthConfig     `koanf:"auth" yaml:"auth"`
	Log      LogConfig      `koanf:"log" yaml:"log"`
}

type ServerConfig struct {
	HTTP HTTPServerConfig `koanf:"http" yaml:"http"`
}

type ClientConfig struct {
	HTTP HTTPClientConfig `koanf:"http" yaml:"http"`
}

type HTTPServerConfig struct {
	Address string `koanf:"address" yaml:"address" env:"SERVER_HTTP_ADDRESS" flag:"server-http-address"`
}

type HTTPClientConfig struct {
	Address string `koanf:"address" yaml:"address" env:"CLIENT_HTTP_ADDRESS" flag:"client-http-address"`
}

type DatabaseConfig struct {
	DSN string `koanf:"dsn" yaml:"dsn" env:"DATABASE_DSN" flag:"database-dsn"`
}

type AuthConfig struct {
	JWTSecret string        `koanf:"jwt_secret" yaml:"jwt_secret" env:"AUTH_JWT_SECRET" flag:"auth-jwt-secret"`
	JWTTTL    time.Duration `koanf:"jwt_ttl" yaml:"jwt_ttl" env:"AUTH_JWT_TTL" flag:"auth-jwt-ttl"`
}

type LogConfig struct {
	Level string `koanf:"level" yaml:"level" env:"LOG_LEVEL" flag:"log-level"`
}
