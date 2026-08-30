package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	applogger "github.com/Radiushina/GophKeeper/internal/domains/logger"
	"go.uber.org/zap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	application, cleanup, err := injectApp()
	if err != nil {
		fatal("inject app", err)
	}
	defer cleanup()

	if err := application.run(ctx); err != nil {
		os.Exit(1)
	}
}

func fatal(msg string, err error) {
	log, logErr := applogger.New("error")
	if logErr != nil {
		os.Exit(1)
	}
	log.Fatal(msg, zap.Error(err))
}
