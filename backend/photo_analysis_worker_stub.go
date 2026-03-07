//go:build !release

package backend

import "go.hasen.dev/vbolt"

// PhotoAnalysisJob represents a photo queued for face analysis
type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
}

// AnalysisWorkerStats holds live stats about the face analysis worker.
type AnalysisWorkerStats struct {
	QueueLength int  `json:"queueLength"`
	IsRunning   bool `json:"isRunning"`
}

// GetAnalysisWorkerStats returns zeroed stats in local builds (face tagging disabled).
func GetAnalysisWorkerStats() AnalysisWorkerStats {
	return AnalysisWorkerStats{}
}

// InitializeAnalysisWorker is a no-op in local builds (face tagging disabled)
func InitializeAnalysisWorker(db *vbolt.DB) {}

// QueuePhotoAnalysis is a no-op in local builds (face tagging disabled)
func QueuePhotoAnalysis(job PhotoAnalysisJob) {}

// TriggerPersonFaceUpdate is a no-op in local builds (face tagging disabled)
func TriggerPersonFaceUpdate(personId int) {}
