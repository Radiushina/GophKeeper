//go:build wireinject
// +build wireinject

package main

import (
	"github.com/google/wire"
)

func injectApp() (*app, func(), error) {
	wire.Build(
		newFlags,
		newConfig,
		newLogger,
		newCLI,
		newApp,
	)
	return nil, nil, nil
}
