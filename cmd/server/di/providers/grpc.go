package providers

import (
	"fmt"
	"net"

	"github.com/Radiushina/GophKeeper/config"
	"github.com/Radiushina/GophKeeper/gen/filepb"
	"github.com/Radiushina/GophKeeper/internal/domains/file"
	"github.com/Radiushina/GophKeeper/internal/domains/user"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// GRPCListen is the file-stream listener + server.
type GRPCListen struct {
	Addr   string
	Server *grpc.Server
}

// NewGRPCServer registers FileService.
func NewGRPCServer(cfg *config.Config, svc *file.Service, jwt *user.JWT, log *zap.Logger) *GRPCListen {
	s := grpc.NewServer(grpc.MaxRecvMsgSize(2 << 20))
	filepb.RegisterFileServiceServer(s, file.NewGRPC(svc, jwt, log))
	return &GRPCListen{Addr: cfg.Server.GRPC.Address, Server: s}
}

func (g *GRPCListen) listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", g.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen %s: %w", g.Addr, err)
	}
	return ln, nil
}
