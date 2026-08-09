// Command restoredrill boots the real application against a restored database
// and exercises the paths a restore has to bring back, then exits.
//
// It is the "does the app actually run on this?" half of the restore drill in
// docs/restore.md; verifydb answers "is the data there?". Running it is the
// only way to confirm the vbolt.ApplyDBProcess migrations in OpenDB re-run
// cleanly against a restored snapshot, since a restore of an older archive
// replays whatever migrations that archive had not yet seen.
//
// Run it with the working directory set to a scratch tree holding
// .serve/db.bolt and .serve/static/photos — a local build resolves both paths
// relative to the working directory (cfg/local.go), so it never touches the
// real database.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	family "family"
	"family/backend"
	"family/cfg"

	"go.hasen.dev/vbolt"
)

func main() {
	replay := flag.Bool("replay-migrations", false,
		"forget the recorded migrations first, so OpenDB re-runs them all")
	flag.Parse()

	if cfg.IsRelease {
		log.Fatal("restoredrill is a local-build tool; release paths point at production")
	}
	if _, err := os.Stat(cfg.DBPath); err != nil {
		log.Fatalf("no database at %s (run from the scratch tree): %v", cfg.DBPath, err)
	}

	if *replay {
		forgetMigrations(cfg.DBPath)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	base := fmt.Sprintf("http://%s", listener.Addr().String())

	// Mirror local/local.go: static data has to be mounted explicitly, and it
	// is the half of the restore being checked when a probe path is a photo.
	app := family.MakeApplication()
	app.StaticData = os.DirFS(cfg.StaticDir)
	handler := backend.NewRequestSizeLimitWrapper(backend.NewSecurityWrapper(app))

	server := family.NewHTTPServer("", handler)
	go func() { _ = server.Serve(listener) }()

	failures := 0
	for _, path := range flag.Args() {
		if err := probe(base + path); err != nil {
			fmt.Printf("FAIL %s: %v\n", path, err)
			failures++
			continue
		}
		fmt.Printf("ok   %s\n", path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)

	if failures > 0 {
		os.Exit(1)
	}
	fmt.Println("\nrestoredrill: OK")
}

// forgetMigrations deletes the DBProcesses records that make ApplyDBProcess a
// no-op, so the next OpenDB replays every migration against restored data.
//
// A restore of today's archive skips all of them — the records came back with
// the snapshot — which means the happy path proves nothing about the case that
// actually bites: restoring an archive taken before a migration landed. This
// forces that case. It has to run before MakeApplication because bolt takes an
// exclusive flock, so the database must be closed again first.
func forgetMigrations(dbPath string) {
	db := vbolt.Open(dbPath)
	vbolt.InitBuckets(db, &cfg.Info)

	var names []string
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, vbolt.DBProcesses, func(name string, _ time.Time) bool {
			names = append(names, name)
			return true
		})
	})
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, name := range names {
			vbolt.Delete(tx, vbolt.DBProcesses, name)
		}
		vbolt.TxCommit(tx)
	})
	db.Close()

	fmt.Printf("forgot %d migration record(s): %v\n\n", len(names), names)
}

func probe(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	if len(body) == 0 {
		return fmt.Errorf("status 200 with an empty body")
	}
	fmt.Printf("     %d bytes, %s\n", len(body), resp.Header.Get("Content-Type"))
	return nil
}
