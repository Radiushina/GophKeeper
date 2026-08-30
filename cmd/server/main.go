package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Radiushina/GophKeeper/cmd/server/di"
	applogger "github.com/Radiushina/GophKeeper/internal/domains/logger"
	"go.uber.org/zap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, cleanup, err := di.InjectApp(ctx)
	if err != nil {
		fatal("inject app", err)
	}
	defer cleanup()

	if err := app.Run(ctx); err != nil {
		app.Log.Fatal("run app", zap.Error(err))
	}
}

func fatal(msg string, err error) {
	log, logErr := applogger.New("error")
	if logErr != nil {
		os.Exit(1)
	}
	log.Fatal(msg, zap.Error(err))
}
