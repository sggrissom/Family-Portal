package family

import (
	"context"
	"errors"
	"family/backend"
	"family/cfg"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

// MigrateMedia copies local video files into the bucket named by MEDIA_S3_*.
// Run it with the app stopped, after setting the bucket variables and before
// starting the app on them.
func MigrateMedia(args []string) error {
	flags := flag.NewFlagSet("migrate-media", flag.ContinueOnError)
	deleteLocal := flags.Bool("delete-local", false, "remove each local file once the bucket holds it at full size")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if cfg.IsRelease {
		_ = godotenv.Load("/srv/apps/family/shared/.env")
	} else {
		_ = godotenv.Load()
	}
	store, err := backend.NewConfiguredS3Store()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result, err := backend.MigrateLocalMedia(ctx, cfg.MediaDir, store, *deleteLocal)
	fmt.Printf("copied %d (%d bytes), already there %d, failed %d, local files removed %d\n",
		result.Copied, result.Bytes, result.Skipped, result.Failed, result.LocalDeleted)
	for _, failure := range result.Failures {
		fmt.Println("  " + failure)
	}
	if err != nil {
		return err
	}
	if result.Failed > 0 {
		return errors.New("some files were not copied; run again to retry them")
	}
	return nil
}
