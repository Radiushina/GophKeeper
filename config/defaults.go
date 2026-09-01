package config

import "time"

func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			HTTP: HTTPServerConfig{
				Address: ":8080",
			},
		},
		Client: ClientConfig{
			HTTP: HTTPClientConfig{
				Address: "http://localhost:8080",
			},
		},
		Auth: AuthConfig{
			JWTTTL: 24 * time.Hour,
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}
