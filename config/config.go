package config

import "time"

type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Client   ClientConfig   `koanf:"client"`
	Database DatabaseConfig `koanf:"database"`
	Auth     AuthConfig     `koanf:"auth"`
	Log      LogConfig      `koanf:"log"`
}

type ServerConfig struct {
	HTTP HTTPServerConfig `koanf:"http"`
}

type ClientConfig struct {
	HTTP HTTPClientConfig `koanf:"http"`
}

type HTTPServerConfig struct {
	Address string `koanf:"address"`
}

type HTTPClientConfig struct {
	Address string `koanf:"address"`
}

type DatabaseConfig struct {
	DSN string `koanf:"dsn"`
}

type AuthConfig struct {
	JWTSecret string        `koanf:"jwt_secret"`
	JWTTTL    time.Duration `koanf:"jwt_ttl"`
}

type LogConfig struct {
	Level string `koanf:"level"`
}
