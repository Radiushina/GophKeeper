package main

import (
	"context"

	"github.com/Radiushina/GophKeeper/internal/client"
)

type app struct {
	cli   *client.App
	flags *Flags
}

func newApp(cli *client.App, flags *Flags) *app {
	return &app{cli: cli, flags: flags}
}

func (a *app) run(ctx context.Context) error {
	return client.Run(ctx, a.cli, a.flags.args)
}
