package providers

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/filepb"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCListen is the file-stream listener + server.
type GRPCListen struct {
	Addr   string
	Server *grpc.Server
}

// NewGRPCServer registers FileService.
func NewGRPCServer(cfg *config.Config, svc *file.Service, jwt *user.JWT, log *zap.Logger) (*GRPCListen, error) {
	// 1. Читаем паспорт сервера (cert) и секретную печать (key) с диска.
	cert, err := tls.LoadX509KeyPair(cfg.Server.GRPC.TLS.CertFile, cfg.Server.GRPC.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load grpc tls key pair: %w", err)
	}
	// 2. tls.Config говорит Go: «при handshake используй этот сертификат».
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12, // не принимать совсем старый TLS
	}
	// 3. credentials.NewTLS — обёртка TLS для gRPC.
	creds := credentials.NewTLS(tlsCfg)
	s := grpc.NewServer(
		grpc.Creds(creds), // ← вместо «открытого» plaintext
		grpc.MaxRecvMsgSize(2<<20),
	)
	filepb.RegisterFileServiceServer(s, file.NewGRPC(svc, jwt, log))
	return &GRPCListen{Addr: cfg.Server.GRPC.Address, Server: s}, nil
}

func (g *GRPCListen) listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", g.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen %s: %w", g.Addr, err)
	}
	return ln, nil
}
