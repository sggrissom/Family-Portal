package backend

import (
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestGetWeeklyDigestCountsOnlyTheWindow(t *testing.T) {
	db := logTestDB(t, "test_weekly_digest.db")
	token := adminContext(t, db)

	now := time.Now()
	inside := now.Add(-2 * 24 * time.Hour)
	outside := now.Add(-30 * 24 * time.Hour)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		lurker := User{Id: 2, Name: "Lurker", Email: "lurker@example.com", Creation: outside, LastLogin: outside}
		uploader := User{Id: 3, Name: "Uploader", Email: "up@example.com", Creation: outside, LastLogin: outside}
		vbolt.Write(tx, UsersBkt, lurker.Id, &lurker)
		vbolt.Write(tx, UsersBkt, uploader.Id, &uploader)

		images := []Image{
			{Id: 1, FamilyId: 1, OwnerUserId: 3, CreatedAt: inside},
			{Id: 2, FamilyId: 1, OwnerUserId: 3, CreatedAt: inside},
			{Id: 3, FamilyId: 1, OwnerUserId: 3, CreatedAt: outside},
		}
		for _, image := range images {
			vbolt.Write(tx, ImagesBkt, image.Id, &image)
		}

		messages := []ChatMessage{
			{Id: 1, FamilyId: 1, UserId: 1, CreatedAt: inside},
			{Id: 2, FamilyId: 1, UserId: 1, CreatedAt: outside},
		}
		for _, message := range messages {
			vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)
		}

		milestone := Milestone{Id: 1, FamilyId: 1, CreatedAt: inside}
		vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)

		growth := GrowthData{Id: 1, FamilyId: 1, CreatedAt: outside}
		vbolt.Write(tx, GrowthDataBkt, growth.Id, &growth)

		vbolt.TxCommit(tx)
	})

	var resp WeeklyDigestResponse
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = GetWeeklyDigest(&vbeam.Context{Tx: tx, Token: token}, Empty{})
		if err != nil {
			t.Fatalf("GetWeeklyDigest() error = %v", err)
		}
	})

	if resp.Photos != 2 || resp.Milestones != 1 || resp.Messages != 1 || resp.Measurements != 0 {
		t.Errorf("totals = %d photos, %d milestones, %d messages, %d measurements; want 2/1/1/0",
			resp.Photos, resp.Milestones, resp.Messages, resp.Measurements)
	}
	if resp.Accounts != 3 {
		t.Errorf("Accounts = %d, want 3", resp.Accounts)
	}
	if resp.Absent != 1 {
		t.Errorf("Absent = %d, want 1 — only Lurker did nothing and signed in to nothing", resp.Absent)
	}
	if resp.Quiet {
		t.Error("Quiet = true in a week with uploads")
	}
	if resp.WindowDays != 7 {
		t.Errorf("WindowDays = %d, want 7", resp.WindowDays)
	}

	if len(resp.People) != 2 {
		t.Fatalf("People = %+v, want the admin and the uploader", resp.People)
	}
	if resp.People[0].Name != "Uploader" || resp.People[0].Photos != 2 {
		t.Errorf("People[0] = %+v, want Uploader with 2 photos first", resp.People[0])
	}
	if resp.People[0].SignedIn {
		t.Error("Uploader.SignedIn = true; the last password login was a month ago")
	}
	if resp.People[1].Messages != 1 {
		t.Errorf("People[1] = %+v, want the admin's one message", resp.People[1])
	}
}

func TestGetWeeklyDigestIsQuietOnAnEmptyWeek(t *testing.T) {
	db := logTestDB(t, "test_weekly_digest_quiet.db")
	token := adminContext(t, db)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		dormant := User{Id: 1, Name: "Admin User", Email: "admin@example.com",
			Creation:  time.Now().Add(-90 * 24 * time.Hour),
			LastLogin: time.Now().Add(-90 * 24 * time.Hour)}
		vbolt.Write(tx, UsersBkt, dormant.Id, &dormant)
		vbolt.TxCommit(tx)
	})

	var resp WeeklyDigestResponse
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var err error
		resp, err = GetWeeklyDigest(&vbeam.Context{Tx: tx, Token: token}, Empty{})
		if err != nil {
			t.Fatalf("GetWeeklyDigest() error = %v", err)
		}
	})

	if !resp.Quiet {
		t.Errorf("Quiet = false on a week with nothing in it: %+v", resp)
	}
	if len(resp.People) != 0 {
		t.Errorf("People = %+v, want none", resp.People)
	}
	if resp.Absent != 1 {
		t.Errorf("Absent = %d, want 1", resp.Absent)
	}
}

func TestGetWeeklyDigestRequiresAdmin(t *testing.T) {
	db := logTestDB(t, "test_weekly_digest_auth.db")
	adminContext(t, db)

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		_, err := GetWeeklyDigest(&vbeam.Context{Tx: tx, Token: ""}, Empty{})
		if err == nil {
			t.Error("GetWeeklyDigest() with no token returned no error")
		}
	})
}
