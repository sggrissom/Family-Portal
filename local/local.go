package main

import (
	"fmt"
	"net/http"
	"os"
	family "family"
	"family/backend"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbeam/esbuilder"
	"go.hasen.dev/vbeam/local_ui"

	"family/cfg"
)

const Port = 8666
const Domain = "family.localhost"
const FEDist = ".serve/frontend"

func StartLocalServer() {
	defer vbeam.NiceStackTraceOnPanic()

	vbeam.RunBackServer(cfg.Backport)
	app := family.MakeApplication()
	app.Frontend = os.DirFS(FEDist)
	app.StaticData = os.DirFS(cfg.StaticDir)
	vbeam.GenerateTSBindings(app, "frontend/server.ts")

	// Wrap with security headers
	secureApp := backend.NewSecurityWrapper(app)

	var addr = fmt.Sprintf(":%d", Port)
	var appServer = &http.Server{Addr: addr, Handler: secureApp}
	appServer.ListenAndServe()
}

var FEOpts = esbuilder.FEBuildOptions{
	FERoot: "frontend",
	EntryTS: []string{
		"main.tsx",
	},
	EntryHTML: []string{"index.html"},
	CopyItems: []string{
		"images",
	},
	Outdir: FEDist,
	Define: map[string]string{
		"BROWSER": "true",
		"DEBUG":   "true",
		"VERBOSE": "false",
	},
}

var FEWatchDirs = []string{
	"frontend",
}

func main() {
	os.MkdirAll(".serve", 0755)
	os.MkdirAll(".serve/static", 0755)
	os.MkdirAll(".serve/frontend", 0755)

	var args local_ui.LocalServerArgs
	args.Domain = Domain
	args.Port = Port
	args.FEOpts = FEOpts
	args.FEWatchDirs = FEWatchDirs
	args.StartServer = StartLocalServer

	local_ui.LaunchUI(args)
}
