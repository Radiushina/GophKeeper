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
				TLS: GRPCServerTLSCfg{
					CertFile: "certs/server.crt",
					KeyFile:  "certs/server.key",
				},
			},
		},
		Client: ClientConfig{
			HTTP: HTTPClientConfig{
				Address: "http://localhost:8080",
			},
			GRPC: GRPCClientConfig{
				Address: "localhost:9091",
				TLS: GRPCClientTLSCfg{
					CAFile:     "certs/ca.crt",
					ServerName: "localhost",
				},
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
