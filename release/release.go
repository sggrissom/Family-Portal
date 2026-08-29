//go:build release || !frontend
// +build release !frontend

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	family "family"
	"family/backend"
	"family/cfg"
)

//go:embed dist
var embedded embed.FS

func main() {
	// run returns rather than calling log.Fatal so that the deferred shutdown
	// in RunHTTPServer actually runs; the exit status is set here instead.
	if err := run(); err != nil {
		log.Printf("server stopped unexpectedly: %v", err)
		os.Exit(1)
	}
}

func run() error {
	// Create required directories
	os.MkdirAll("data", 0755)
	os.MkdirAll("static", 0755)

	distFS, err := fs.Sub(embedded, "dist")
	if err != nil {
		return fmt.Errorf("failed to sub-fs the embedded frontend: %w", err)
	}

	// Create the application with frontend assets
	app := family.MakeApplication()
	app.Frontend = distFS
	app.StaticData = os.DirFS(cfg.StaticDir)

	// Security headers, request size limits, and rate limiting
	handler := family.WrapApplication(app)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("listening on %s\n", addr)
	var appServer = family.NewHTTPServer(addr, handler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go backend.RunTokenCleanup(ctx, app.DB)
	return family.RunHTTPServer(ctx, appServer)
}
