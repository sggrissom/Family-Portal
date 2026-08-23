package backend

import (
	"family/cfg"
	"log"
	"os"
	"path/filepath"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// LogFileBaseName is the stem of the file the rotating logger writes to. The
// log viewer needs the same name to recognise the current day's file; it used
// to carry its own literal, which had drifted, so the "today" badge never lit
// on the one file you always want.
const LogFileBaseName = "family_record"

// LogFileName is the current log file as it appears on disk.
const LogFileName = LogFileBaseName + ".log"

// LogFilePath is the current log file, in whichever directory this build logs to.
func LogFilePath() string {
	return filepath.Join(cfg.LogDir, LogFileName)
}

// InitRotatingLogger points the standard logger at a rotating file in
// cfg.LogDir. It replaces vbeam.InitRotatingLogger, which is identical except
// that it hardcodes a relative "logs/" — and relative meant *inside the release
// directory*, since the systemd unit sets WorkingDirectory to
// /srv/apps/family/current. Every deploy therefore started an empty log, and
// the sixth deploy after an incident pruned the evidence away. The window you
// most want is right after a deploy that broke something, and the deploy that
// broke it was the one that reset the log.
//
// cfg.LogDir is under shared/ in a release build, which survives deploys by
// design and is already where the database and photos live.
func InitRotatingLogger() {
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		// Not fatal: logging to stderr is worse than logging to a file, but it
		// is a great deal better than refusing to start.
		log.Printf("Warning: could not create log directory %s: %v", cfg.LogDir, err)
		return
	}

	logger := &lumberjack.Logger{
		Filename:   LogFilePath(),
		MaxSize:    200, // megabytes
		MaxBackups: 10,
		MaxAge:     10, // days
		LocalTime:  true,
	}

	// Rotate on the midnight boundary, so a day's traffic is one file and the
	// viewer's date-named files mean what they look like they mean.
	go func() {
		for {
			now := time.Now()
			nextDay := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
			time.Sleep(nextDay.Sub(now))
			logger.Rotate()
		}
	}()

	log.SetOutput(logger)
}
