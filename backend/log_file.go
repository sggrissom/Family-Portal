package backend

import (
	"family/cfg"
	"log"
	"os"
	"path/filepath"
	"time"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const LogFileBaseName = "family_record"

const LogFileName = LogFileBaseName + ".log"

func LogFilePath() string {
	return filepath.Join(cfg.LogDir, LogFileName)
}

func InitRotatingLogger() {
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		log.Printf("Warning: could not create log directory %s: %v", cfg.LogDir, err)
		return
	}

	logger := &lumberjack.Logger{
		Filename:   LogFilePath(),
		MaxSize:    200,
		MaxBackups: 10,
		MaxAge:     10,
		LocalTime:  true,
	}

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
