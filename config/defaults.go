package config

import "time"

func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			HTTP: HTTPServerConfig{
				Address: ":8080",
			},
			GRPC: GRPCServerConfig{
				Address: ":9091",
			},
		},
		Client: ClientConfig{
			HTTP: HTTPClientConfig{
				Address: "http://localhost:8080",
			},
			GRPC: GRPCClientConfig{
				Address: "localhost:9091",
			},
		},
		Auth: AuthConfig{
			JWTTTL: 24 * time.Hour,
		},
		Log: LogConfig{
			Level: "info",
		},
		S3: S3Config{
			Region:    "us-east-1",
			PathStyle: true,
		},
	}
}
