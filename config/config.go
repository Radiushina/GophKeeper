package config

import "time"

type Config struct {
	Server   ServerConfig   `koanf:"server" yaml:"server"`
	Client   ClientConfig   `koanf:"client" yaml:"client"`
	Database DatabaseConfig `koanf:"database" yaml:"database"`
	Auth     AuthConfig     `koanf:"auth" yaml:"auth"`
	Log      LogConfig      `koanf:"log" yaml:"log"`
	S3       S3Config       `koanf:"s3" yaml:"s3"`
}

type ServerConfig struct {
	HTTP HTTPServerConfig `koanf:"http" yaml:"http"`
	GRPC GRPCServerConfig `koanf:"grpc" yaml:"grpc"`
}

type ClientConfig struct {
	HTTP HTTPClientConfig `koanf:"http" yaml:"http"`
	GRPC GRPCClientConfig `koanf:"grpc" yaml:"grpc"`
}

type GRPCServerConfig struct {
	Address string `koanf:"address" yaml:"address" env:"SERVER_GRPC_ADDRESS" flag:"server-grpc-address"`
}

type GRPCClientConfig struct {
	Address string `koanf:"address" yaml:"address" env:"CLIENT_GRPC_ADDRESS" flag:"client-grpc-address"`
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

type S3Config struct {
	Endpoint  string `koanf:"endpoint" yaml:"endpoint" env:"S3_ENDPOINT" flag:"s3-endpoint"`
	Region    string `koanf:"region" yaml:"region" env:"S3_REGION" flag:"s3-region"`
	Bucket    string `koanf:"bucket" yaml:"bucket" env:"S3_BUCKET" flag:"s3-bucket"`
	AccessKey string `koanf:"access_key" yaml:"access_key" env:"S3_ACCESS_KEY" flag:"s3-access-key"`
	SecretKey string `koanf:"secret_key" yaml:"secret_key" env:"S3_SECRET_KEY" flag:"s3-secret-key"`
	PathStyle bool   `koanf:"path_style" yaml:"path_style" env:"S3_PATH_STYLE" flag:"s3-path-style"`
}
