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
	"strings"

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
	var images []backend.Image

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

		vbolt.IterateAll(tx, backend.ImagesBkt, func(_ int, img backend.Image) bool {
			images = append(images, img)
			return true
		})
	})
	counts["images"] = len(images)

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
		failures += checkOriginals(*staticDir, images)
	} else if len(images) > 0 {
		fmt.Printf("\n%d image rows not checked against disk (pass -static to verify originals)\n", len(images))
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

// originalPath mirrors how photo_worker.go names the file it keeps for backup:
// the upload's unique basename with an "_original" suffix before the original
// extension (backend/photo_worker.go:221-227).
func originalPath(staticDir, filePath string) string {
	base := filepath.Join(staticDir, filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "_original" + ext
}

// checkOriginals reports image rows whose original is not on disk. Originals
// are the only photo files the backup carries — every other variant is
// regenerable — so a missing one is unrecoverable data loss, not a warning.
//
// The reverse direction, an original with no row, is only reported. It means
// the archive is carrying a photo the app can no longer reach, which wastes
// space but loses nothing; DeletePhoto removes the original alongside the row
// (backend/photos.go:1224), so a leftover is a delete that did not finish.
func checkOriginals(staticDir string, images []backend.Image) int {
	referenced := make(map[string]bool, len(images))
	var missing []backend.Image
	for _, img := range images {
		path := originalPath(staticDir, img.FilePath)
		referenced[filepath.Base(path)] = true
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, img)
		}
	}

	fmt.Printf("\noriginals: %d/%d present under %s\n", len(images)-len(missing), len(images), staticDir)

	entries, err := os.ReadDir(filepath.Join(staticDir, "photos"))
	if err == nil {
		var orphans []string
		for _, entry := range entries {
			name := entry.Name()
			if strings.Contains(name, "_original.") && !referenced[name] {
				orphans = append(orphans, name)
			}
		}
		if len(orphans) > 0 {
			fmt.Printf("\n%d original(s) on disk with no image row (harmless, but backed up anyway):\n", len(orphans))
			for _, name := range orphans {
				fmt.Printf("  %s\n", name)
			}
		}
	}

	if len(missing) == 0 {
		return 0
	}

	fmt.Printf("\nFAIL: %d image row(s) have no original on disk:\n", len(missing))
	for _, img := range missing {
		fmt.Printf("  id=%d status=%d family=%d %s\n", img.Id, img.Status, img.FamilyId, img.FilePath)
	}
	return 1
}
