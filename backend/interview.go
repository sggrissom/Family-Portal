package backend

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"family/cfg"
	"fmt"
	"slices"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const CategoryInterview = "interview"

const (
	InterviewConducted  = "conducted"
	InterviewHistorical = "historical"

	InterviewInProgress = "in-progress"
	InterviewComplete   = "complete"
)

var ErrInterviewCategory = errors.New("Interview answers can only be changed from the interview")

type InterviewQuestionSnapshot struct {
	QuestionId int    `json:"questionId"`
	Text       string `json:"text"`
	Section    string `json:"section"`
}

func PackInterviewQuestionSnapshot(self *InterviewQuestionSnapshot, buf *vpack.Buffer) {
	vpack.Int(&self.QuestionId, buf)
	vpack.String(&self.Text, buf)
	vpack.String(&self.Section, buf)
}

type Interview struct {
	AnswerCount int                         `json:"answerCount"`
	Id          int                         `json:"id"`
	FamilyId    int                         `json:"familyId"`
	PersonId    int                         `json:"personId"`
	Date        time.Time                   `json:"date"`
	Title       string                      `json:"title"`
	Source      string                      `json:"source"`
	Status      string                      `json:"status"`
	QuestionIds []int                       `json:"questionIds"`
	SkippedIds  []int                       `json:"skippedIds"`
	PhotoId     int                         `json:"photoId"`
	Notes       string                      `json:"notes"`
	CreatedAt   time.Time                   `json:"createdAt"`
	CompletedAt time.Time                   `json:"completedAt"`
	Version     int                         `json:"version"`
	Questions   []InterviewQuestionSnapshot `json:"questions,omitempty"`
	OriginKey   string                      `json:"originKey,omitempty"`
}

func PackInterview(self *Interview, buf *vpack.Buffer) {
	version := vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Time(&self.Date, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Source, buf)
	vpack.String(&self.Status, buf)
	vpack.Slice(&self.QuestionIds, vpack.Int, buf)
	vpack.Slice(&self.SkippedIds, vpack.Int, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.String(&self.Notes, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.CompletedAt, buf)
	vpack.Int(&self.Version, buf)
	if version >= 2 {
		vpack.Slice(&self.Questions, PackInterviewQuestionSnapshot, buf)
		vpack.String(&self.OriginKey, buf)
	}
}

var InterviewBkt = vbolt.Bucket(&cfg.Info, "interviews", vpack.FInt, PackInterview)
var InterviewByPersonIndex = vbolt.Index(&cfg.Info, "interviews_by_person", vpack.FInt, vpack.FInt)
var InterviewByFamilyIndex = vbolt.Index(&cfg.Info, "interviews_by_family", vpack.FInt, vpack.FInt)

func GetInterviewById(tx *vbolt.Tx, id int) (interview Interview) {
	vbolt.Read(tx, InterviewBkt, id, &interview)
	return
}

func writeInterview(tx *vbolt.Tx, interview Interview) {
	vbolt.Write(tx, InterviewBkt, interview.Id, &interview)
	vbolt.SetTargetSingleTerm(tx, InterviewByPersonIndex, interview.Id, interview.PersonId)
	vbolt.SetTargetSingleTerm(tx, InterviewByFamilyIndex, interview.Id, interview.FamilyId)
}

func readInterviews(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], key int) (interviews []Interview) {
	var ids []int
	vbolt.ReadTermTargets(tx, index, key, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, InterviewBkt, ids, &interviews)
	}
	return
}

func familyInterviews(tx *vbolt.Tx, familyId int) []Interview {
	return readInterviews(tx, InterviewByFamilyIndex, familyId)
}

func personInterviews(tx *vbolt.Tx, personId int) []Interview {
	return readInterviews(tx, InterviewByPersonIndex, personId)
}

func interviewAnswers(tx *vbolt.Tx, interviewId int) (answers []Milestone) {
	var ids []int
	vbolt.ReadTermTargets(tx, MilestoneByInterviewIndex, interviewId, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, ids, &answers)
	}
	return
}

func questionAnswers(tx *vbolt.Tx, questionId int) (answers []Milestone) {
	var ids []int
	vbolt.ReadTermTargets(tx, MilestoneByQuestionIndex, questionId, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, ids, &answers)
	}
	return
}

func replaceId(ids []int, from, into int) []int {
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id == from {
			id = into
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// Old records have a stable identity too; unlike dates, record ids distinguish sittings.
func interviewOrigin(i Interview) string {
	if i.OriginKey != "" {
		return i.OriginKey
	}
	return fmt.Sprintf("legacy:%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", i.FamilyId, i.Id, i.CreatedAt.UTC().Format(time.RFC3339Nano)))))
}

func newInterviewOrigin() string { return rand.Text() }

// Freeze the best available historical wording before a legacy sitting is mutated.
func snapshotInterview(tx *vbolt.Tx, i *Interview) {
	if len(i.Questions) > 0 {
		return
	}
	person := GetPersonById(tx, i.PersonId)
	i.Questions = []InterviewQuestionSnapshot{}
	for _, id := range i.QuestionIds {
		q := GetInterviewQuestionById(tx, id)
		text := renderQuestion(q.Text, person, i.Date)
		if a := answerFor(tx, i.Id, id); a.Id > 0 && a.Context != "" {
			text = a.Context
		}
		i.Questions = append(i.Questions, InterviewQuestionSnapshot{QuestionId: id, Text: text, Section: q.Section})
	}
}

func interviewPrompt(tx *vbolt.Tx, i Interview, id int) string {
	snapshotInterview(tx, &i)
	for _, q := range i.Questions {
		if q.QuestionId == id {
			return q.Text
		}
	}
	return ""
}

// Freeze legacy prompts once at deployment, before later list/birthday edits.
func BackfillInterviewSnapshots(tx *vbolt.Tx) {
	updates := []Interview{}
	vbolt.IterateAll(tx, InterviewBkt, func(_ int, i Interview) bool {
		if (len(i.Questions) == 0 && len(i.QuestionIds) > 0) || i.OriginKey == "" {
			snapshotInterview(tx, &i)
			i.OriginKey = interviewOrigin(i)
			updates = append(updates, i)
		}
		return true
	})
	for _, i := range updates {
		writeInterview(tx, i)
	}
}
