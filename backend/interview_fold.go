package backend

import (
	"fmt"
	"time"

	"go.hasen.dev/vbolt"
)

// interviewsOfAnswers loads the interviews that the given answers belong to,
// so a client can fold them into one card with the right title and portrait.
func interviewsOfAnswers(tx *vbolt.Tx, milestones []Milestone) []Interview {
	interviews := []Interview{}
	seen := map[int]bool{}
	for _, m := range milestones {
		if m.InterviewId == 0 || seen[m.InterviewId] {
			continue
		}
		seen[m.InterviewId] = true
		if interview := GetInterviewById(tx, m.InterviewId); interview.Id > 0 {
			interviews = append(interviews, interview)
		}
	}
	return interviews
}

func removePhotoFromInterviews(tx *vbolt.Tx, photo Image) {
	for _, interview := range familyInterviews(tx, photo.FamilyId) {
		if interview.PhotoId == photo.Id {
			interview.PhotoId = 0
			writeInterview(tx, interview)
		}
	}
}

func movePersonInterviewsTx(tx *vbolt.Tx, fromPersonId, toPersonId int) {
	for _, interview := range personInterviews(tx, fromPersonId) {
		interview.PersonId = toPersonId
		writeInterview(tx, interview)
	}
}

func deletePersonInterviewsTx(tx *vbolt.Tx, personId int) {
	for _, interview := range personInterviews(tx, personId) {
		deleteInterviewTx(tx, interview)
	}
}

func deleteFamilyInterviewsTx(tx *vbolt.Tx, familyId int) {
	for _, interview := range familyInterviews(tx, familyId) {
		deleteInterviewTx(tx, interview)
	}
	for _, q := range familyQuestions(tx, familyId) {
		deleteInterviewQuestion(tx, q.Id)
	}
	vbolt.Delete(tx, InterviewQuestionListBkt, familyId)
}

func interviewNudge(tx *vbolt.Tx, person Person, interviews []Interview, today time.Time) (rankedNudge, bool) {
	name := firstName(person.Name)
	for _, interview := range interviews {
		if interview.Status != InterviewInProgress {
			continue
		}
		answered := len(interviewAnswers(tx, interview.Id))
		return rankedNudge{DashboardNudge{
			Kind:        "interview-continue",
			Key:         fmt.Sprintf("interview-continue:%d", interview.Id),
			Text:        fmt.Sprintf("Continue %s's interview · %d of %d", name, answered, len(interview.QuestionIds)),
			PersonId:    person.Id,
			Count:       answered,
			InterviewId: interview.Id,
		}, 2}, true
	}
	if !interviewDue(person, interviews, today) {
		return rankedNudge{}, false
	}
	birthday := person.Birthday.UTC()
	year := today.Year()
	if today.Month() == time.January && birthday.Month() == time.December {
		year--
	} else if today.Month() == time.December && birthday.Month() == time.January {
		year++
	}
	return rankedNudge{DashboardNudge{
		Kind:     "interview",
		Key:      fmt.Sprintf("interview:%d:%d", person.Id, year),
		Text:     fmt.Sprintf("🎤 Time for %s's annual interview", name),
		PersonId: person.Id,
	}, 4}, true
}
