package backend

import (
	"sort"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterInterviewViewMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetPersonInterviews)
	vbeam.RegisterProc(app, GetFamilyInterviews)
	vbeam.RegisterProc(app, CompareInterviewAnswers)
}

const (
	interviewDueBeforeDays = 14
	interviewDueAfterDays  = 30
	interviewMinAgeYears   = 3
	interviewRecentMonths  = 10
	adultAgeYears          = 18
)

func sortInterviewsByDate(interviews []Interview) {
	sort.Slice(interviews, func(i, j int) bool {
		if !interviews[i].Date.Equal(interviews[j].Date) {
			return interviews[i].Date.Before(interviews[j].Date)
		}
		return interviews[i].Id < interviews[j].Id
	})
}

// interviewDue is true around a child's birthday when they haven't had an
// interview in the last ten months.
func interviewDue(person Person, interviews []Interview, today time.Time) bool {
	if person.IsPregnancy || person.Birthday.IsZero() {
		return false
	}
	today = dayStart(today)
	birthday := person.Birthday.UTC()
	near := time.Time{}
	for _, year := range []int{today.Year() - 1, today.Year(), today.Year() + 1} {
		candidate := time.Date(year, birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
		days := int(candidate.Sub(today).Hours() / 24)
		if days <= interviewDueBeforeDays && days >= -interviewDueAfterDays {
			near = candidate
			break
		}
	}
	if near.IsZero() || ageYearsAt(birthday, near) < interviewMinAgeYears {
		return false
	}
	recent := today.AddDate(0, -interviewRecentMonths, 0)
	for _, interview := range interviews {
		if interview.Date.After(recent) {
			return false
		}
	}
	return true
}

type GetPersonInterviewsRequest struct {
	PersonId int `json:"personId"`
}

type GetPersonInterviewsResponse struct {
	Person     Person              `json:"person"`
	Interviews []Interview         `json:"interviews"`
	Answers    []Milestone         `json:"answers"`
	Questions  []InterviewQuestion `json:"questions"`
}

func personInterviewBundle(tx *vbolt.Tx, person Person) GetPersonInterviewsResponse {
	resp := GetPersonInterviewsResponse{
		Person:     person,
		Interviews: personInterviews(tx, person.Id),
		Answers:    []Milestone{},
		Questions:  []InterviewQuestion{},
	}
	if resp.Interviews == nil {
		resp.Interviews = []Interview{}
	}
	sortInterviewsByDate(resp.Interviews)

	seen := map[int]bool{}
	addQuestion := func(id int) {
		if seen[id] {
			return
		}
		seen[id] = true
		if q := GetInterviewQuestionById(tx, id); q.Id > 0 {
			resp.Questions = append(resp.Questions, q)
		}
	}
	for _, interview := range resp.Interviews {
		for _, id := range interview.QuestionIds {
			addQuestion(id)
		}
		for _, m := range interviewAnswers(tx, interview.Id) {
			addQuestion(m.QuestionId)
			resp.Answers = append(resp.Answers, m)
		}
	}
	sortQuestions(resp.Questions)
	return resp
}

func GetPersonInterviews(ctx *vbeam.Context, req GetPersonInterviewsRequest) (resp GetPersonInterviewsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
		err = ErrPersonNotFound
		return
	}
	resp = personInterviewBundle(ctx.Tx, person)
	return
}

type GetFamilyInterviewsRequest struct {
	Today string `json:"today"`
}

type FamilyInterviewRow struct {
	Person     Person    `json:"person"`
	Last       Interview `json:"last"`
	Count      int       `json:"count"`
	InProgress Interview `json:"inProgress"`
	Answered   int       `json:"answered"`
	Due        bool      `json:"due"`
	Child      bool      `json:"child"`
}

type GetFamilyInterviewsResponse struct {
	People []FamilyInterviewRow `json:"people"`
}

func familyInterviewRow(tx *vbolt.Tx, person Person, today time.Time) FamilyInterviewRow {
	interviews := personInterviews(tx, person.Id)
	sortInterviewsByDate(interviews)
	row := FamilyInterviewRow{
		Person: person,
		Due:    interviewDue(person, interviews, today),
		Child:  !person.IsPregnancy && !person.Birthday.IsZero() && ageYearsAt(person.Birthday, today) < adultAgeYears,
	}
	for _, interview := range interviews {
		if interview.Status == InterviewInProgress {
			row.InProgress = interview
			row.Answered = len(interviewAnswers(tx, interview.Id))
			continue
		}
		row.Count++
		row.Last = interview
	}
	return row
}

func GetFamilyInterviews(ctx *vbeam.Context, req GetFamilyInterviewsRequest) (resp GetFamilyInterviewsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	today := time.Now().UTC()
	if parsed, parseErr := time.Parse("2006-01-02", req.Today); parseErr == nil {
		today = parsed
	}
	resp.People = []FamilyInterviewRow{}
	for _, person := range GetVisiblePeople(ctx.Tx, user) {
		if person.IsPregnancy || !CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
			continue
		}
		resp.People = append(resp.People, familyInterviewRow(ctx.Tx, person, today))
	}
	sort.SliceStable(resp.People, func(i, j int) bool {
		a, b := resp.People[i], resp.People[j]
		if a.Child != b.Child {
			return a.Child
		}
		return a.Person.Birthday.Before(b.Person.Birthday)
	})
	return
}

type CompareInterviewAnswersRequest struct {
	QuestionId int   `json:"questionId"`
	PersonIds  []int `json:"personIds"`
}

type CompareRow struct {
	Person  Person           `json:"person"`
	Answers []PreviousAnswer `json:"answers"`
}

type CompareInterviewAnswersResponse struct {
	Question  InterviewQuestion   `json:"question"`
	Questions []InterviewQuestion `json:"questions"`
	Rows      []CompareRow        `json:"rows"`
}

// CompareInterviewAnswers lays one question's answers out by age for each
// person, so siblings (or a parent's own childhood interview) line up.
func CompareInterviewAnswers(ctx *vbeam.Context, req CompareInterviewAnswersRequest) (resp CompareInterviewAnswersResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId := 0
	if req.QuestionId > 0 {
		familyId = GetInterviewQuestionById(ctx.Tx, req.QuestionId).FamilyId
	} else if familyId, err = ResolveActingFamily(ctx.Tx, user, 0, AccessView); err != nil {
		return
	}

	visible := map[int]Person{}
	canSee := func(personId int) (Person, bool) {
		if p, ok := visible[personId]; ok {
			return p, p.Id > 0
		}
		p := GetPersonById(ctx.Tx, personId)
		if !CanAccessPerson(ctx.Tx, user, p, ScopeMilestones, AccessView) {
			p = Person{}
		}
		visible[personId] = p
		return p, p.Id > 0
	}

	resp.Questions = []InterviewQuestion{}
	for _, q := range familyQuestions(ctx.Tx, familyId) {
		for _, m := range questionAnswers(ctx.Tx, q.Id) {
			if _, ok := canSee(m.PersonId); ok {
				resp.Questions = append(resp.Questions, q)
				break
			}
		}
	}
	if len(resp.Questions) == 0 {
		if req.QuestionId > 0 {
			err = ErrInterviewQuestionNotFound
		}
		resp.Rows = []CompareRow{}
		return
	}
	for _, q := range resp.Questions {
		if q.Id == req.QuestionId {
			resp.Question = q
		}
	}
	if resp.Question.Id == 0 {
		if req.QuestionId > 0 {
			err = ErrInterviewQuestionNotFound
			return
		}
		resp.Question = resp.Questions[0]
	}

	wanted := map[int]bool{}
	for _, id := range req.PersonIds {
		wanted[id] = true
	}
	rows := map[int]*CompareRow{}
	var order []int
	for _, m := range questionAnswers(ctx.Tx, resp.Question.Id) {
		person, ok := canSee(m.PersonId)
		if !ok || (len(wanted) > 0 && !wanted[person.Id]) {
			continue
		}
		row := rows[person.Id]
		if row == nil {
			row = &CompareRow{Person: person, Answers: []PreviousAnswer{}}
			rows[person.Id] = row
			order = append(order, person.Id)
		}
		row.Answers = append(row.Answers, PreviousAnswer{
			InterviewId: m.InterviewId, MilestoneId: m.Id, Date: m.MilestoneDate,
			AgeYears: ageYearsAt(person.Birthday, m.MilestoneDate), Text: m.Description,
		})
	}
	resp.Rows = []CompareRow{}
	for _, id := range order {
		row := rows[id]
		sort.Slice(row.Answers, func(i, j int) bool { return row.Answers[i].Date.Before(row.Answers[j].Date) })
		resp.Rows = append(resp.Rows, *row)
	}
	sort.SliceStable(resp.Rows, func(i, j int) bool {
		return resp.Rows[i].Person.Birthday.Before(resp.Rows[j].Person.Birthday)
	})
	return
}
