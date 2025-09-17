package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"go.hasen.dev/vbeam/esbuilder"
)

func main() {
	log.Println("Starting frontend build…")

	if _, err := os.Stat("frontend"); os.IsNotExist(err) {
		log.Fatal("Error: frontend directory does not exist")
	}

	if _, err := os.Stat("frontend/main.tsx"); os.IsNotExist(err) {
		log.Fatal("Error: frontend/main.tsx does not exist")
	}

	if _, err := os.Stat("frontend/index.html"); os.IsNotExist(err) {
		log.Fatal("Error: frontend/index.html does not exist")
	}

	reportCh := make(chan esbuilder.ESReport, 2)

	options := esbuilder.FEBuildOptions{
		FERoot:       "frontend",
		EntryTS:      []string{"main.tsx"},
		EntryHTML:    []string{"index.html"},
		CopyItems:    []string{},
		Outdir:       "release/dist",
		NoSourceMaps: true,
		Define: map[string]string{
			"BROWSER": "true",
			"DEBUG":   "false",
			"VERBOSE": "false",
		},
	}

	log.Printf("Build options: FERoot=%s, EntryTS=%v, Outdir=%s\n",
		options.FERoot, options.EntryTS, options.Outdir)

	ok := esbuilder.FEBuild(options, reportCh)

	report := <-reportCh

	if !ok {
		log.Println("Build failed: FEBuild returned false")
		os.Exit(1)
	}

	if len(report.Errors) > 0 {
		log.Printf("Build completed with %d error(s):\n", len(report.Errors))
		for i, e := range report.Errors {
			log.Printf("Error %d: %s\n", i+1, e.Text)
			if e.Location.File != "" {
				log.Printf("  File: %s\n", e.Location.File)
				log.Printf("  Line: %d, Column: %d\n", e.Location.Line, e.Location.Column)
			}
			if e.Location.LineText != "" {
				log.Printf("  Code: %s\n", e.Location.LineText)
			}
		}
		os.Exit(1)
	}

	fmt.Printf(" Built into release/dist in %s (started at %s)\n",
		report.Duration.Truncate(time.Millisecond),
		report.Time.Format("15:04:05"),
	)
}
