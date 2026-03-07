//go:build !release

package backend

import "go.hasen.dev/vbolt"

// PhotoAnalysisJob represents a photo queued for face analysis
type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
}

// InitializeAnalysisWorker is a no-op in local builds (face tagging disabled)
func InitializeAnalysisWorker(db *vbolt.DB) {}

// QueuePhotoAnalysis is a no-op in local builds (face tagging disabled)
func QueuePhotoAnalysis(job PhotoAnalysisJob) {}

// TriggerPersonFaceUpdate is a no-op in local builds (face tagging disabled)
func TriggerPersonFaceUpdate(personId int) {}
