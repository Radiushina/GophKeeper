package di

import (
	"context"

	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/config"
	"go.uber.org/zap"
)

type App struct {
	cfg    *config.Config
	server *providers.Servers
	Log    *zap.Logger
}

func (a *App) Run(ctx context.Context) error {
	a.server.Start(ctx)
	return nil
}
