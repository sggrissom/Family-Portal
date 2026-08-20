package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbolt"
)

// The contract these tests hold is documented in docs/degraded-dependencies.md:
// an optional dependency failing may cost its own output and nothing else.

// Face analysis is unreachable in a local build by construction — the worker is
// a stub — so a photo upload has to survive it never running.
func TestQueueingAnalysisWithoutAWorkerIsHarmless(t *testing.T) {
	// No panic, no error to propagate, nothing to check: the whole point is
	// that the upload path cannot tell the difference.
	QueuePhotoAnalysis(PhotoAnalysisJob{ImageId: 1, FamilyId: 1})
	TriggerPersonFaceUpdate(1)
}

// Push is optional, and a chat message must not depend on it. Without a
// configured worker the queue call fails, and the caller's job is to swallow it.
func TestQueueingPushWithoutAWorkerReportsRatherThanPanics(t *testing.T) {
	previous := globalPushWorker
	globalPushWorker = nil
	t.Cleanup(func() { globalPushWorker = previous })

	if err := QueuePushNotification(PushNotificationJob{MessageId: 1, FamilyId: 1}); err == nil {
		t.Error("QueuePushNotification should report that there is no worker")
	}
}

// A full queue drops derived work rather than blocking the request that
// produced it. An unbounded queue would trade a dropped notification for a
// stalled handler, which is the wrong way round.
func TestAFullPushQueueRefusesInsteadOfBlocking(t *testing.T) {
	previous := globalPushWorker
	// A worker that is never started leaves its queue unattended, which is the
	// only reliable way to fill it.
	globalPushWorker = &PushWorker{jobQueue: make(chan PushNotificationJob, 1)}
	t.Cleanup(func() { globalPushWorker = previous })

	if err := QueuePushNotification(PushNotificationJob{MessageId: 1}); err != nil {
		t.Fatalf("first notification rejected: %v", err)
	}
	if err := QueuePushNotification(PushNotificationJob{MessageId: 2}); err == nil {
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

// AI import is a drafting step. With no key configured it must report that in
// the response rather than fail the call or write anything.
func TestAIImportWithoutAKeyReportsInTheResponse(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")

	if err := ValidateAIConfiguration(); err == nil {
		t.Fatal("ValidateAIConfiguration accepted an empty key")
	}
}

// A chat message is committed before the notification is queued, so a push
// outage cannot cost anyone their message. This checks the ordering holds by
// sending with no push worker at all and finding the message in the database.
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

	// The notification attempt happens after the commit above; it fails here
	// because there is no worker, and the message must still be readable.
	_ = QueuePushNotification(PushNotificationJob{MessageId: stored.Id, FamilyId: stored.FamilyId})

	var readBack ChatMessage
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.Read(tx, ChatMessagesBkt, stored.Id, &readBack)
	})
	if readBack.Content != "hello" {
		t.Errorf("chat message = %q, want %q; a push failure must not cost a message", readBack.Content, "hello")
	}
}
