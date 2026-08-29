package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Radiushina/GophKeeper/cmd/server/di"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, cleanup, err := di.InjectApp(ctx)
	if err != nil {
		log.Fatalf("inject app: %v", err)
	}
	defer cleanup()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
