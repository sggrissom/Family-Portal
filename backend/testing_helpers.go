package backend

import (
	"os"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// setupTestApp creates a test application with temporary database file
func setupTestApp(t *testing.T) (*vbeam.Application, func()) {
	tempFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	db := vbolt.Open(tempPath)
	app := vbeam.NewApplication("TestApp", db)

	return app, func() {
		db.Close()
		os.Remove(tempPath)
	}
}
