package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbolt"
)

func TestQueueingAnalysisWithoutAWorkerIsHarmless(t *testing.T) {
	QueuePhotoAnalysis(PhotoAnalysisJob{ImageId: 1, FamilyId: 1})
	TriggerPersonFaceUpdate(1)
}

func TestQueueingPushWithoutAWorkerReportsRatherThanPanics(t *testing.T) {
	previous := globalPushWorker
	globalPushWorker = nil
	t.Cleanup(func() { globalPushWorker = previous })

	if err := QueuePushNotification(PushNotificationJob{Event: PushEventChatMessage, RecordId: 1, FamilyId: 1}); err == nil {
		t.Error("QueuePushNotification should report that there is no worker")
	}
}

func TestAFullPushQueueRefusesInsteadOfBlocking(t *testing.T) {
	previous := globalPushWorker
	globalPushWorker = &PushWorker{jobQueue: make(chan PushNotificationJob, 1)}
	t.Cleanup(func() { globalPushWorker = previous })

	if err := QueuePushNotification(PushNotificationJob{Event: PushEventChatMessage, RecordId: 1}); err != nil {
		t.Fatalf("first notification rejected: %v", err)
	}
	if err := QueuePushNotification(PushNotificationJob{Event: PushEventChatMessage, RecordId: 2}); err == nil {
		t.Error("QueuePushNotification blocked or accepted a job with no room")
	}
}

func TestAFullPhotoQueueRefusesInsteadOfBlocking(t *testing.T) {
	previous := globalPhotoWorker
	globalPhotoWorker = &PhotoWorker{jobQueue: make(chan PhotoProcessingJob, 1)}
	t.Cleanup(func() { globalPhotoWorker = previous })

	if err := QueuePhotoProcessing(PhotoProcessingJob{ImageId: 1}); err != nil {
		t.Fatalf("first photo rejected: %v", err)
	}
	if err := QueuePhotoProcessing(PhotoProcessingJob{ImageId: 2}); err == nil {
		t.Error("QueuePhotoProcessing blocked or accepted a job with no room")
	}
}

func TestAIImportWithoutAKeyReportsInTheResponse(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")

	if err := ValidateAIConfiguration(); err == nil {
		t.Fatal("ValidateAIConfiguration accepted an empty key")
	}
}

func TestChatMessageSurvivesAnUnavailablePushWorker(t *testing.T) {
	dbPath := "test_degraded_chat.db"
	db := vbolt.Open(dbPath)
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	vbolt.InitBuckets(db, &cfg.Info)

	previous := globalPushWorker
	globalPushWorker = nil
	t.Cleanup(func() { globalPushWorker = previous })

	var stored ChatMessage
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		stored = ChatMessage{Id: 1, FamilyId: 1, UserId: 1, UserName: "Test", Content: "hello"}
		vbolt.Write(tx, ChatMessagesBkt, stored.Id, &stored)
		vbolt.TxCommit(tx)
	})

	_ = QueuePushNotification(PushNotificationJob{Event: PushEventChatMessage, RecordId: stored.Id, FamilyId: stored.FamilyId})

	var readBack ChatMessage
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.Read(tx, ChatMessagesBkt, stored.Id, &readBack)
	})
	if readBack.Content != "hello" {
		t.Errorf("chat message = %q, want %q; a push failure must not cost a message", readBack.Content, "hello")
	}
}
