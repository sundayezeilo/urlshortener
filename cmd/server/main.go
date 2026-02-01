package main

import (
	"context"
	"log"

	"github.com/sundayezeilo/urlshortener/internal/app"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// Initialize application
	application, err := app.New(ctx)
	if err != nil {
		return err
	}
	defer application.Shutdown()

	// Start server (blocks until shutdown)
	return application.Start(ctx)
}
