package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	family "family"
	"family/backend"
	"family/cfg"
)

//go:embed dist
var embedded embed.FS

const Port = 8666

func main() {
	// Create required directories
	os.MkdirAll("data", 0755)
	os.MkdirAll("static", 0755)

	distFS, err := fs.Sub(embedded, "dist")
	if err != nil {
		log.Fatalf("failed to sub‐fs: %v", err)
	}

	// Create the application with frontend assets
	app := family.MakeApplication()
	app.Frontend = distFS
	app.StaticData = os.DirFS(cfg.StaticDir)

	// Wrap with security headers
	secureApp := backend.NewSecurityWrapper(app)

	addr := fmt.Sprintf(":%d", Port)
	log.Printf("listening on %s\n", addr)
	var appServer = &http.Server{Addr: addr, Handler: secureApp}
	appServer.ListenAndServe()
}
