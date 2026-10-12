package backend

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
)

func TestInterviewsTravelWithTheirPeople(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	answer := fx.answer(t, got.Interview.Id, got.Questions[0].Id, "pizza", false).Answer

	timeline, err := callAs(t, fx, GetFamilyTimeline, GetFamilyTimelineRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range timeline.People {
		if item.Person.Id == fx.alice.Id && (len(item.Interviews) != 1 || item.Interviews[0].Id != got.Interview.Id) {
			t.Errorf("timeline interviews for alice = %+v", item.Interviews)
		}
	}
	person, _ := callAs(t, fx, GetPerson, GetPersonRequest{Id: fx.alice.Id})
	if len(person.Interviews) != 1 {
		t.Errorf("GetPerson interviews = %+v", person.Interviews)
	}

	if _, err := callAs(t, fx, MergePeople, MergePeopleRequest{SourcePersonId: fx.alice.Id, TargetPersonId: fx.bob.Id}); err != nil {
		t.Fatal(err)
	}
	moved, err := callAs(t, fx, GetInterview, GetInterviewRequest{Id: got.Interview.Id})
	if err != nil || moved.Interview.PersonId != fx.bob.Id || len(moved.Answers) != 1 || moved.Answers[0].PersonId != fx.bob.Id {
		t.Fatalf("after merge: %+v, %v", moved, err)
	}

	summary, _ := callAs(t, fx, GetPersonDeletionSummary, PersonDeletionRequest{PersonId: fx.bob.Id})
	if summary.Interviews != 1 {
		t.Errorf("deletion summary counts %d interviews, want 1", summary.Interviews)
	}
	if _, err := callAs(t, fx, DeletePerson, PersonDeletionRequest{PersonId: fx.bob.Id}); err != nil {
		t.Fatal(err)
	}
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetInterviewById(tx, got.Interview.Id).Id != 0 || GetMilestoneById(tx, answer.Id).Id != 0 {
			t.Error("deleting the person left the interview or its answer")
		}
	})
}

func TestDeletingAFamilyRemovesItsQuestions(t *testing.T) {
	fx := setupResultsFixture(t)
	fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deleteFamilyContentTx(tx, fx.familyId)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if n := len(familyQuestions(tx, fx.familyId)); n != 0 {
			t.Errorf("%d questions left", n)
		}
		if n := len(familyInterviews(tx, fx.familyId)); n != 0 {
			t.Errorf("%d interviews left", n)
		}
		if familyQuestionList(tx, fx.familyId).Seeded {
			t.Error("the list record survived")
		}
	})
}

func TestDeletingAPortraitClearsIt(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	photo := fx.addPhoto(t, fx.familyId, "portrait.jpg")
	fx.finish(t, got.Interview)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		interview := GetInterviewById(tx, got.Interview.Id)
		interview.PhotoId = photo.Id
		writeInterview(tx, interview)
		deletePhotoRecordTx(tx, photo)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if GetInterviewById(tx, got.Interview.Id).PhotoId != 0 {
			t.Error("the interview still points at a deleted photo")
		}
	})
}

func TestInterviewNudges(t *testing.T) {
	fx := setupResultsFixture(t)
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }

	var due, none rankedNudge
	var dueOk, noneOk bool
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		due, dueOk = interviewNudge(tx, fx.alice, nil, day("2025-02-20"))
		none, noneOk = interviewNudge(tx, fx.alice, nil, day("2025-06-20"))
	})
	if !dueOk || due.Kind != "interview" || due.Key != fmt.Sprintf("interview:%d:2025", fx.alice.Id) {
		t.Errorf("due nudge = %+v, %v", due, dueOk)
	}
	if noneOk {
		t.Errorf("nudged away from the birthday: %+v", none)
	}

	got := fx.startInterview(t, fx.alice, "2025-02-20", "fav-food", "fav-color")
	fx.answer(t, got.Interview.Id, got.Questions[0].Id, "pizza", false)
	var cont rankedNudge
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		cont, _ = interviewNudge(tx, fx.alice, personInterviews(tx, fx.alice.Id), day("2025-02-21"))
	})
	if cont.Kind != "interview-continue" || cont.InterviewId != got.Interview.Id || !strings.Contains(cont.Text, "1 of 2") {
		t.Errorf("continue nudge = %+v", cont)
	}
}

func TestInterviewAnswersEmbedWithTheirQuestion(t *testing.T) {
	answer := Milestone{Category: CategoryInterview, Context: "What's your favorite food?", Description: "pizza"}
	if got := milestoneEmbedText(answer); got != "What's your favorite food?: pizza" {
		t.Errorf("embed text = %q", got)
	}
	quote := Milestone{Category: "quote", Context: "At dinner", Description: "I'm a dinosaur"}
	if got := milestoneEmbedText(quote); got != "I'm a dinosaur" {
		t.Errorf("a quote's embed text changed to %q", got)
	}
	if milestoneEmbeddingOutdated(quote, MilestoneEmbedding{Model: sentenceModelId, Version: milestoneEmbedVersion, Text: quote.Description, Vector: []float32{1}}, true) {
		t.Error("other moments would re-embed")
	}
	if !milestoneEmbeddingOutdated(answer, MilestoneEmbedding{Model: sentenceModelId, Version: milestoneEmbedVersion, Text: "pizza", Vector: []float32{1}}, true) {
		t.Error("an answer embedded from its text alone isn't re-embedded")
	}
}

func TestNormalizingRecordDatesCoversInterviews(t *testing.T) {
	fx := setupResultsFixture(t)
	got := fx.startInterview(t, fx.alice, "2021-03-05", "fav-food")
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		interview := GetInterviewById(tx, got.Interview.Id)
		interview.Date = time.Date(2021, 3, 5, 6, 0, 0, 0, time.FixedZone("CST", -6*3600))
		writeInterview(tx, interview)
		NormalizeStoredRecordDates(tx)
		vbolt.TxCommit(tx)
	})
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if d := GetInterviewById(tx, got.Interview.Id).Date; !d.Equal(time.Date(2021, 3, 5, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("date = %v", d)
		}
	})
}
