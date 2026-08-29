package di

import (
	"context"

	"github.com/Radiushina/GophKeeper/cmd/server/di/providers"
	"github.com/Radiushina/GophKeeper/config"
)

type App struct {
	cfg    *config.Config
	server *providers.Servers
}

func (a *App) Run(ctx context.Context) error {
	a.server.Start(ctx)
	return nil
}
