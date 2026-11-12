package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const (
	CadenceBirthday    = "birthday"
	CadenceSchoolStart = "school-start"
	CadenceNewYear     = "new-year"
	CadenceOff         = "off"
	defaultSchoolMonth = 8
)

// InterviewCadence is when a family's interviews are suggested. Month is the
// month school starts, and is only set for CadenceSchoolStart.
type InterviewCadence struct {
	Kind  string `json:"kind"`
	Month int    `json:"month"`
}

func (list InterviewQuestionList) cadence() InterviewCadence {
	switch list.Cadence {
	case CadenceSchoolStart:
		month := list.CadenceMonth
		if month < 1 || month > 12 {
			month = defaultSchoolMonth
		}
		return InterviewCadence{Kind: CadenceSchoolStart, Month: month}
	case CadenceNewYear, CadenceOff:
		return InterviewCadence{Kind: list.Cadence}
	default:
		return InterviewCadence{Kind: CadenceBirthday}
	}
}

func familyInterviewCadence(tx *vbolt.Tx, familyId int) InterviewCadence {
	return familyQuestionList(tx, familyId).cadence()
}

func exportInterviewCadence(tx *vbolt.Tx, familyId int) *InterviewCadence {
	list := familyQuestionList(tx, familyId)
	if list.Cadence == "" {
		return nil
	}
	cadence := list.cadence()
	return &cadence
}

// importInterviewCadence takes the exported cadence only when the family
// hasn't chosen one of its own.
func importInterviewCadence(tx *vbolt.Tx, familyId int, exported *InterviewCadence) {
	if exported == nil {
		return
	}
	cadence, err := validCadence(*exported)
	list := familyQuestionList(tx, familyId)
	if err != nil || list.Cadence != "" {
		return
	}
	list.Cadence = cadence.Kind
	list.CadenceMonth = cadence.Month
	writeQuestionList(tx, list)
}

var ErrInvalidCadence = errors.New("Choose when interviews should be suggested")

func validCadence(c InterviewCadence) (InterviewCadence, error) {
	switch c.Kind {
	case CadenceBirthday, CadenceNewYear, CadenceOff:
		return InterviewCadence{Kind: c.Kind}, nil
	case CadenceSchoolStart:
		if c.Month < 1 || c.Month > 12 {
			return c, ErrInvalidCadence
		}
		return c, nil
	}
	return c, ErrInvalidCadence
}

// interviewOccasion is the date in the family's cadence that today falls
// near, for the year it falls in.
func interviewOccasion(person Person, cadence InterviewCadence, today time.Time) (time.Time, bool) {
	birthday := person.Birthday.UTC()
	for _, year := range []int{today.Year() - 1, today.Year(), today.Year() + 1} {
		var candidate time.Time
		switch cadence.Kind {
		case CadenceBirthday:
			candidate = time.Date(year, birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
		case CadenceSchoolStart:
			candidate = time.Date(year, time.Month(cadence.Month), 1, 0, 0, 0, 0, time.UTC)
		case CadenceNewYear:
			candidate = time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		default:
			return time.Time{}, false
		}
		days := int(candidate.Sub(today).Hours() / 24)
		if days <= interviewDueBeforeDays && days >= -interviewDueAfterDays {
			return candidate, true
		}
	}
	return time.Time{}, false
}

type SetInterviewCadenceRequest struct {
	FamilyId int              `json:"familyId,omitempty"`
	Cadence  InterviewCadence `json:"cadence"`
}

type SetInterviewCadenceResponse struct {
	Cadence InterviewCadence `json:"cadence"`
}

func SetInterviewCadence(ctx *vbeam.Context, req SetInterviewCadenceRequest) (resp SetInterviewCadenceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}
	cadence, err := validCadence(req.Cadence)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	list := familyQuestionList(ctx.Tx, familyId)
	list.Cadence = cadence.Kind
	list.CadenceMonth = cadence.Month
	writeQuestionList(ctx.Tx, list)
	resp.Cadence = list.cadence()
	vbolt.TxCommit(ctx.Tx)
	return
}
