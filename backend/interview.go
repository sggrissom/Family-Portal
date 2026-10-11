package backend

import (
	"errors"
	"family/cfg"
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

type Interview struct {
	Id          int       `json:"id"`
	FamilyId    int       `json:"familyId"`
	PersonId    int       `json:"personId"`
	Date        time.Time `json:"date"`
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	QuestionIds []int     `json:"questionIds"`
	SkippedIds  []int     `json:"skippedIds"`
	PhotoId     int       `json:"photoId"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt"`
	Version     int       `json:"version"`
}

func PackInterview(self *Interview, buf *vpack.Buffer) {
	vpack.Version(1, buf)
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
