//go:build !release

package backend

import (
	"context"

	"go.hasen.dev/vbolt"
)

type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
}

type AnalysisWorkerStats struct {
	QueueLength int  `json:"queueLength"`
	IsRunning   bool `json:"isRunning"`
}

func GetAnalysisWorkerStats() AnalysisWorkerStats {
	return AnalysisWorkerStats{}
}

func InitializeAnalysisWorker(db *vbolt.DB) {}

func QueuePhotoAnalysis(job PhotoAnalysisJob) {}

func TriggerPersonFaceUpdate(personId int) {}

func StopAnalysisWorker(ctx context.Context) bool { return true }
