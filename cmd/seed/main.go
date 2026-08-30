//go:build !release

// Command seed fills a development database with a large, coherent family so
// the site has something to show without anyone clicking through hundreds of
// forms. It writes ten accounts across five households, a few thousand
// measurements, and the links and roster rows that make cross-family
// permissions observable.
//
//	make seed-fresh   # throw away .serve/db.bolt and build it again
//	make seed         # seed in place, refusing a database that already has users
//
// The credentials are printed at the end of every run and every account shares
// one password (backend.SeedPassword). Both the seeder and this command are
// excluded from release builds, so none of that can reach a deployed binary.
//
// bolt takes an exclusive lock on the database file, so `make local` has to be
// stopped first. A run that cannot take the lock says so rather than hanging.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	family "family"
	"family/backend"
	"family/cfg"

	"go.hasen.dev/vbolt"
)

func main() {
	dbPath := flag.String("db", cfg.DBPath, "database file to seed")
	reset := flag.Bool("reset", false, "delete the database file first, so the seed lands in a fresh one")
	scale := flag.Int("scale", 1, "measurement density multiplier; 2 records twice as often")
	password := flag.String("password", backend.SeedPassword, "password for every seeded account")
	flag.Parse()

	if cfg.IsRelease {
		fmt.Fprintln(os.Stderr, "seed: refusing to run against a release configuration")
		os.Exit(1)
	}

	if *reset {
		if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "seed: could not remove %s: %v\n", *dbPath, err)
			os.Exit(1)
		}
		fmt.Printf("removed %s\n", *dbPath)
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}

	db := open(*dbPath)
	defer db.Close()

	if existing := userCount(db); existing > 0 {
		fmt.Fprintf(os.Stderr,
			"seed: %s already holds %d account(s). Re-run with -reset to replace it.\n",
			*dbPath, existing)
		os.Exit(1)
	}

	var summary backend.SeedSummary
	var seedErr error
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		summary, seedErr = backend.SeedDemoData(tx, backend.SeedOptions{
			Password: *password,
			Scale:    *scale,
		})
		if seedErr != nil {
			return
		}
		vbolt.TxCommit(tx)
	})
	if seedErr != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", seedErr)
		os.Exit(1)
	}

	report(*dbPath, *password, summary)
}

// open translates bolt's lock timeout into the advice that actually resolves
// it, because "timeout" on its own does not point at the running dev server.
func open(dbPath string) (db *vbolt.DB) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr,
				"seed: could not open %s: %v\n"+
					"Another process holds the database. Stop `make local` and try again.\n",
				dbPath, r)
			os.Exit(1)
		}
	}()
	// OpenDB rather than a bare vbolt.Open, so the schema migrations are
	// recorded as applied and the dev server does not re-run them over data
	// that was already written in their final shape.
	return family.OpenDB(dbPath)
}

func userCount(db *vbolt.DB) (count int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, backend.UsersBkt, func(int, backend.User) bool {
			count++
			return true
		})
	})
	return
}

func report(dbPath, password string, summary backend.SeedSummary) {
	fmt.Printf("\nSeeded %s\n\n", dbPath)

	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(out, "EMAIL\tNAME\tFAMILY\tACCESS")
	for _, account := range summary.Accounts {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", account.Email, account.Name, account.Family, account.Access)
	}
	out.Flush()

	fmt.Printf("\nPassword for every account: %s\n", password)

	counts := []struct {
		label string
		value int
	}{
		{"families", summary.Families},
		{"people", summary.People},
		{"relations", summary.Relations},
		{"family links", summary.Links},
		{"shared roster rows", summary.Shares},
		{"tags", summary.Tags},
		{"milestones", summary.Milestones},
		{"measurements", summary.Measurements},
		{"activities", summary.Activities},
		{"activity events", summary.Events},
		{"activity results", summary.Results},
		{"chat messages", summary.ChatMessages},
	}
	var parts []string
	for _, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count.value, count.label))
	}
	fmt.Printf("\n%s\n\nRun `make local` and sign in at http://localhost:8666\n",
		strings.Join(parts, ", "))
}
