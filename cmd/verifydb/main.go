// Command verifydb inspects a Family Portal database file and reports whether
// it holds the data a restore is supposed to bring back.
//
// It exists for the restore drill in docs/restore.md: after `backupctl fetch`
// puts an archive on disk, "did the restore work?" needs an answer that is not
// "the app started". This counts every durable bucket and cross-checks the
// image rows against the photo originals sitting next to them, so a database
// that restored fine alongside a photo tree that did not is a loud failure
// rather than a discovery months later.
//
// Point it at a *copy*. bolt takes an exclusive flock, so it cannot open the
// live production database while the server is running, and opening a database
// runs vbolt.InitBuckets, which writes.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"family/backend"
	"family/cfg"

	"go.hasen.dev/vbolt"
)

func main() {
	dbPath := flag.String("db", "", "path to a db.bolt copy (required)")
	staticDir := flag.String("static", "", "path to the static/ directory holding photos/ (optional)")
	flag.Parse()

	if *dbPath == "" {
		fmt.Fprintln(os.Stderr, "usage: verifydb -db <db.bolt> [-static <static dir>]")
		os.Exit(2)
	}
	if _, err := os.Stat(*dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "verifydb: %v\n", err)
		os.Exit(1)
	}

	db := vbolt.Open(*dbPath)
	defer db.Close()
	vbolt.InitBuckets(db, &cfg.Info)

	counts := map[string]int{}
	var report backend.PhotoConsistencyReport

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		counts["users"] = count(tx, backend.UsersBkt)
		counts["families"] = count(tx, backend.FamiliesBkt)
		counts["people"] = count(tx, backend.PeopleBkt)
		counts["person_family"] = count(tx, backend.PersonFamilyBkt)
		counts["family_membership"] = count(tx, backend.FamilyMembershipBkt)
		counts["growth_data"] = count(tx, backend.GrowthDataBkt)
		counts["milestones"] = count(tx, backend.MilestoneBkt)
		counts["milestone_photos"] = count(tx, backend.MilestonePhotoBkt)
		counts["milestone_tags"] = count(tx, backend.MilestoneTagBkt)
		counts["tags"] = count(tx, backend.TagBkt)
		counts["photo_tags"] = count(tx, backend.PhotoTagBkt)
		counts["photo_person"] = count(tx, backend.PhotoPersonBkt)
		counts["chat_messages"] = count(tx, backend.ChatMessagesBkt)
		counts["family_link"] = count(tx, backend.FamilyLinkBkt)
		counts["activities"] = count(tx, backend.ActivityBkt)
		counts["seasons"] = count(tx, backend.SeasonBkt)
		counts["activity_events"] = count(tx, backend.EventBkt)
		counts["activity_entries"] = count(tx, backend.EntryBkt)
		counts["entry_members"] = count(tx, backend.EntryMemberBkt)
		counts["appearances"] = count(tx, backend.AppearanceBkt)
		counts["activity_results"] = count(tx, backend.ResultBkt)
		counts["appearance_photos"] = count(tx, backend.AppearancePhotoBkt)
		counts["activity_event_photos"] = count(tx, backend.EventPhotoBkt)

		report = backend.ScanPhotoConsistency(tx, *staticDir, 0)
	})
	counts["images"] = report.TotalImages

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("database: %s\n\n", *dbPath)
	for _, name := range names {
		fmt.Printf("  %-22s %6d\n", name, counts[name])
	}

	// A restored database with zero people is a restored empty file. Nothing
	// downstream would notice, so say so here.
	failures := 0
	if counts["users"] == 0 || counts["people"] == 0 {
		fmt.Printf("\nFAIL: database holds no users or no people; this is an empty restore\n")
		failures++
	}

	if *staticDir != "" {
		failures += reportOriginals(*staticDir, report)
	} else if report.TotalImages > 0 {
		fmt.Printf("\n%d image rows not checked against disk (pass -static to verify originals)\n", report.TotalImages)
	}

	if failures > 0 {
		fmt.Printf("\nverifydb: %d check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Printf("\nverifydb: OK\n")
}

func count[K, T any](tx *vbolt.Tx, bkt *vbolt.BucketInfo[K, T]) int {
	n := 0
	vbolt.IterateAll(tx, bkt, func(K, T) bool {
		n++
		return true
	})
	return n
}

// A missing original is a failure: originals are the only photo files the backup
// carries. An orphaned original only wastes archive space, so it is reported.
func reportOriginals(staticDir string, report backend.PhotoConsistencyReport) int {
	fmt.Printf("\noriginals: %d/%d present under %s\n", report.PresentCount, report.TotalImages, staticDir)

	if report.OrphanScanErr != "" {
		fmt.Printf("\ncould not scan %s for orphaned originals: %s\n", filepath.Join(staticDir, "photos"), report.OrphanScanErr)
	}

	if report.OrphanCount > 0 {
		fmt.Printf("\n%d original(s) on disk with no image row (harmless, but backed up anyway):\n", report.OrphanCount)
		for _, orphan := range report.Orphans {
			fmt.Printf("  %s\n", orphan.Name)
		}
	}

	if report.MissingCount == 0 {
		return 0
	}

	fmt.Printf("\nFAIL: %d image row(s) have no original on disk:\n", report.MissingCount)
	for _, img := range report.Missing {
		fmt.Printf("  id=%d status=%d family=%d %s\n", img.ImageId, img.Status, img.FamilyId, img.FilePath)
	}
	return 1
}
