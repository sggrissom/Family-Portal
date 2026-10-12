package backend

import (
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterInterviewMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetInterviewSetup)
	vbeam.RegisterProc(app, StartInterview)
	vbeam.RegisterProc(app, GetInterview)
	vbeam.RegisterProc(app, SaveInterviewAnswer)
	vbeam.RegisterProc(app, AddInterviewQuestion)
	vbeam.RegisterProc(app, UpdateInterview)
	vbeam.RegisterProc(app, DeleteInterview)
}

const (
	maxInterviewQuestions = 300
	maxInterviewAnswer    = 2000
	maxInterviewTitle     = 120
	maxInterviewNotes     = 4000
	oneOffSection         = "Also asked"
)

var ErrInterviewNotFound = errors.New("Interview not found or not in your family")
var ErrInterviewInProgress = errors.New("There's already an interview in progress for this person")

func ageYearsAt(birthday, date time.Time) int {
	if birthday.IsZero() || date.Before(birthday) {
		return 0
	}
	years := date.Year() - birthday.Year()
	if date.Month() < birthday.Month() || (date.Month() == birthday.Month() && date.Day() < birthday.Day()) {
		years--
	}
	return years
}

func renderQuestion(text string, person Person, date time.Time) string {
	text = strings.ReplaceAll(text, "{name}", firstName(person.Name))
	return strings.ReplaceAll(text, "{age}", strconv.Itoa(ageYearsAt(person.Birthday, date)))
}

func interviewFor(tx *vbolt.Tx, user User, id int, need AccessLevel) (Interview, Person, error) {
	interview := GetInterviewById(tx, id)
	if interview.Id == 0 || !CanAccessRecordOfPerson(tx, user, interview.FamilyId, interview.PersonId, ScopeMilestones, need) {
		return interview, Person{}, ErrInterviewNotFound
	}
	return interview, GetPersonById(tx, interview.PersonId), nil
}

func inProgressInterview(tx *vbolt.Tx, personId int) Interview {
	for _, interview := range personInterviews(tx, personId) {
		if interview.Status == InterviewInProgress {
			return interview
		}
	}
	return Interview{}
}

func answerFor(tx *vbolt.Tx, interviewId, questionId int) Milestone {
	for _, m := range interviewAnswers(tx, interviewId) {
		if m.QuestionId == questionId {
			return m
		}
	}
	return Milestone{}
}

func deleteInterviewTx(tx *vbolt.Tx, interview Interview) {
	for _, m := range interviewAnswers(tx, interview.Id) {
		DeleteMilestoneTx(tx, m.Id, m.FamilyId)
	}
	vbolt.Delete(tx, InterviewBkt, interview.Id)
	vbolt.DeleteTargetTerms(tx, InterviewByPersonIndex, interview.Id)
	vbolt.DeleteTargetTerms(tx, InterviewByFamilyIndex, interview.Id)
}

func setInterviewDate(tx *vbolt.Tx, interview *Interview, date time.Time) {
	interview.Date = dayStart(date)
	for _, m := range interviewAnswers(tx, interview.Id) {
		if !m.MilestoneDate.Equal(interview.Date) {
			m.MilestoneDate = interview.Date
			writeMilestone(tx, m)
		}
	}
}

func parseInterviewDate(date string, birthday time.Time) (time.Time, error) {
	if strings.TrimSpace(date) == "" {
		return dayStart(time.Now().UTC()), nil
	}
	day, err := resolveEntryDate("Interview", "date", &date, nil, nil, birthday)
	if err != nil {
		return day, errors.New("Interview date must look like 2025-05-03")
	}
	return dayStart(day), nil
}

func createOneOffQuestion(tx *vbolt.Tx, familyId int, text string) (InterviewQuestion, error) {
	text, err := cleanQuestionText(text)
	if err != nil {
		return InterviewQuestion{}, err
	}
	return createInterviewQuestion(tx, InterviewQuestion{
		FamilyId: familyId, Text: text, Section: oneOffSection, OneOff: true, Position: maxFamilyQuestions,
	}), nil
}

// Setup

type GetInterviewSetupRequest struct {
	PersonId int `json:"personId"`
}

type GetInterviewSetupResponse struct {
	Person     Person              `json:"person"`
	Questions  []InterviewQuestion `json:"questions"`
	Retired    []InterviewQuestion `json:"retired"`
	InProgress Interview           `json:"inProgress"`
	Answered   int                 `json:"answered"`
}

func GetInterviewSetup(ctx *vbeam.Context, req GetInterviewSetupRequest) (resp GetInterviewSetupResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute)
	if err != nil {
		return
	}
	resp.Person = GetPersonById(ctx.Tx, req.PersonId)
	resp.Questions = []InterviewQuestion{}
	resp.Retired = []InterviewQuestion{}
	for _, q := range interviewQuestionsResponse(ctx.Tx, familyId).Questions {
		if q.Archived {
			resp.Retired = append(resp.Retired, q)
		} else {
			resp.Questions = append(resp.Questions, q)
		}
	}
	resp.InProgress = inProgressInterview(ctx.Tx, req.PersonId)
	if resp.InProgress.Id > 0 {
		resp.Answered = len(interviewAnswers(ctx.Tx, resp.InProgress.Id))
	}
	return
}

// Start

type StartInterviewRequest struct {
	PersonId        int      `json:"personId"`
	Date            string   `json:"date"`
	QuestionIds     []int    `json:"questionIds"`
	DefaultKeys     []string `json:"defaultKeys"`
	OneOffQuestions []string `json:"oneOffQuestions"`
}

type InterviewResponse struct {
	Interview Interview `json:"interview"`
}

func startInterviewTx(tx *vbolt.Tx, person Person, req StartInterviewRequest) (Interview, error) {
	if inProgressInterview(tx, person.Id).Id > 0 {
		return Interview{}, ErrInterviewInProgress
	}
	date, err := parseInterviewDate(req.Date, person.Birthday)
	if err != nil {
		return Interview{}, err
	}
	ensureFamilyQuestions(tx, person.FamilyId)

	byId := map[int]InterviewQuestion{}
	byKey := map[string]InterviewQuestion{}
	for _, q := range familyQuestions(tx, person.FamilyId) {
		byId[q.Id] = q
		if q.DefaultKey != "" {
			byKey[q.DefaultKey] = q
		}
	}

	interview := Interview{
		FamilyId: person.FamilyId, PersonId: person.Id, Date: date,
		Source: InterviewConducted, Status: InterviewInProgress,
		QuestionIds: []int{}, SkippedIds: []int{}, CreatedAt: time.Now(), Version: 1,
	}
	add := func(id int) {
		if !slices.Contains(interview.QuestionIds, id) {
			interview.QuestionIds = append(interview.QuestionIds, id)
		}
	}
	for _, id := range req.QuestionIds {
		if _, ok := byId[id]; !ok {
			return Interview{}, ErrInterviewQuestionNotFound
		}
		add(id)
	}
	for _, key := range req.DefaultKeys {
		q, ok := byKey[key]
		if !ok {
			return Interview{}, ErrInterviewQuestionNotFound
		}
		add(q.Id)
	}
	for _, text := range req.OneOffQuestions {
		q, err := createOneOffQuestion(tx, person.FamilyId, text)
		if err != nil {
			return Interview{}, err
		}
		add(q.Id)
	}
	if len(interview.QuestionIds) == 0 {
		return Interview{}, errors.New("Pick at least one question")
	}
	if len(interview.QuestionIds) > maxInterviewQuestions {
		return Interview{}, errors.New("That's too many questions for one interview")
	}

	interview.Id = vbolt.NextIntId(tx, InterviewBkt)
	writeInterview(tx, interview)
	return interview, nil
}

func StartInterview(ctx *vbeam.Context, req StartInterviewRequest) (resp InterviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if _, err = ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute); err != nil {
		return
	}
	vbeam.UseWriteTx(ctx)
	resp.Interview, err = startInterviewTx(ctx.Tx, GetPersonById(ctx.Tx, req.PersonId), req)
	if err != nil {
		return
	}
	vbolt.TxCommit(ctx.Tx)
	return
}

// Read

type GetInterviewRequest struct {
	Id int `json:"id"`
}

type PreviousAnswer struct {
	InterviewId int       `json:"interviewId"`
	MilestoneId int       `json:"milestoneId"`
	Date        time.Time `json:"date"`
	AgeYears    int       `json:"ageYears"`
	Text        string    `json:"text"`
}

type GetInterviewResponse struct {
	Interview Interview                `json:"interview"`
	Person    Person                   `json:"person"`
	Questions []InterviewQuestion      `json:"questions"`
	Answers   []Milestone              `json:"answers"`
	Previous  map[int][]PreviousAnswer `json:"previous"`
}

func interviewQuestionsFor(tx *vbolt.Tx, interview Interview, person Person) []InterviewQuestion {
	questions := make([]InterviewQuestion, 0, len(interview.QuestionIds))
	for _, id := range interview.QuestionIds {
		q := GetInterviewQuestionById(tx, id)
		if q.Id == 0 {
			continue
		}
		q.Text = renderQuestion(q.Text, person, interview.Date)
		questions = append(questions, q)
	}
	return questions
}

func previousAnswers(tx *vbolt.Tx, interview Interview, person Person) map[int][]PreviousAnswer {
	previous := map[int][]PreviousAnswer{}
	for _, id := range interview.QuestionIds {
		for _, m := range questionAnswers(tx, id) {
			if m.PersonId != person.Id || m.InterviewId == interview.Id || !m.MilestoneDate.Before(interview.Date) {
				continue
			}
			previous[id] = append(previous[id], PreviousAnswer{
				InterviewId: m.InterviewId, MilestoneId: m.Id, Date: m.MilestoneDate,
				AgeYears: ageYearsAt(person.Birthday, m.MilestoneDate), Text: m.Description,
			})
		}
		sort.Slice(previous[id], func(i, j int) bool { return previous[id][i].Date.After(previous[id][j].Date) })
	}
	return previous
}

func GetInterview(ctx *vbeam.Context, req GetInterviewRequest) (resp GetInterviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	interview, person, err := interviewFor(ctx.Tx, user, req.Id, AccessView)
	if err != nil {
		return
	}
	resp.Interview = interview
	resp.Person = person
	resp.Questions = interviewQuestionsFor(ctx.Tx, interview, person)
	resp.Answers = interviewAnswers(ctx.Tx, interview.Id)
	if resp.Answers == nil {
		resp.Answers = []Milestone{}
	}
	for i := range resp.Answers {
		resp.Answers[i].PhotoIds = GetMilestonePhotoIds(ctx.Tx, resp.Answers[i].Id)
		resp.Answers[i].TagIds = GetMilestoneTagIds(ctx.Tx, resp.Answers[i].Id)
	}
	resp.Previous = previousAnswers(ctx.Tx, interview, person)
	return
}

// Answer

type SaveInterviewAnswerRequest struct {
	InterviewId int    `json:"interviewId"`
	QuestionId  int    `json:"questionId"`
	Answer      string `json:"answer"`
	Skipped     bool   `json:"skipped"`
}

type SaveInterviewAnswerResponse struct {
	Interview Interview `json:"interview"`
	Answer    Milestone `json:"answer"`
}

func saveInterviewAnswerTx(tx *vbolt.Tx, interview Interview, person Person, req SaveInterviewAnswerRequest) (Interview, Milestone, error) {
	if !slices.Contains(interview.QuestionIds, req.QuestionId) {
		return interview, Milestone{}, ErrInterviewQuestionNotFound
	}
	text := strings.TrimSpace(req.Answer)
	if len([]rune(text)) > maxInterviewAnswer {
		return interview, Milestone{}, errors.New("That answer is too long")
	}

	existing := answerFor(tx, interview.Id, req.QuestionId)
	interview.SkippedIds = slices.DeleteFunc(interview.SkippedIds, func(id int) bool { return id == req.QuestionId })
	if req.Skipped {
		interview.SkippedIds = append(interview.SkippedIds, req.QuestionId)
		text = ""
	}

	var answer Milestone
	switch {
	case text == "" && existing.Id > 0:
		DeleteMilestoneTx(tx, existing.Id, existing.FamilyId)
	case text != "" && existing.Id > 0:
		answer = existing
		answer.Description = text
		writeMilestone(tx, answer)
	case text != "":
		q := GetInterviewQuestionById(tx, req.QuestionId)
		answer = Milestone{
			Id:            vbolt.NextIntId(tx, MilestoneBkt),
			PersonId:      interview.PersonId,
			FamilyId:      interview.FamilyId,
			Description:   text,
			Category:      CategoryInterview,
			Context:       renderQuestion(q.Text, person, interview.Date),
			MilestoneDate: interview.Date,
			CreatedAt:     time.Now(),
			InterviewId:   interview.Id,
			QuestionId:    q.Id,
		}
		writeMilestone(tx, answer)
	}
	writeInterview(tx, interview)
	return interview, answer, nil
}

func SaveInterviewAnswer(ctx *vbeam.Context, req SaveInterviewAnswerRequest) (resp SaveInterviewAnswerResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	interview, person, err := interviewFor(ctx.Tx, user, req.InterviewId, AccessContribute)
	if err != nil {
		return
	}
	resp.Interview, resp.Answer, err = saveInterviewAnswerTx(ctx.Tx, interview, person, req)
	if err != nil {
		return
	}
	if resp.Answer.Id > 0 {
		resp.Answer.PhotoIds = GetMilestonePhotoIds(ctx.Tx, resp.Answer.Id)
		resp.Answer.TagIds = GetMilestoneTagIds(ctx.Tx, resp.Answer.Id)
	}
	vbolt.TxCommit(ctx.Tx)
	if resp.Answer.Id > 0 {
		QueueMilestoneEmbedding(resp.Answer.Id)
	}
	return
}

// Follow-up questions

type AddInterviewQuestionRequest struct {
	InterviewId     int    `json:"interviewId"`
	Text            string `json:"text"`
	AfterQuestionId int    `json:"afterQuestionId"`
	Keep            bool   `json:"keep"`
}

type AddInterviewQuestionResponse struct {
	Interview Interview         `json:"interview"`
	Question  InterviewQuestion `json:"question"`
}

func addInterviewQuestionTx(tx *vbolt.Tx, interview Interview, req AddInterviewQuestionRequest) (Interview, InterviewQuestion, error) {
	if len(interview.QuestionIds) >= maxInterviewQuestions {
		return interview, InterviewQuestion{}, errors.New("That's too many questions for one interview")
	}
	var q InterviewQuestion
	var err error
	if req.Keep {
		ensureFamilyQuestions(tx, interview.FamilyId)
		text, textErr := cleanQuestionText(req.Text)
		if textErr != nil {
			return interview, q, textErr
		}
		after := GetInterviewQuestionById(tx, req.AfterQuestionId)
		section := defaultSectionName
		if after.FamilyId == interview.FamilyId && !after.OneOff && after.Section != "" {
			section = after.Section
		}
		q = createInterviewQuestion(tx, InterviewQuestion{FamilyId: interview.FamilyId, Text: text, Section: section})
		list := familyQuestions(tx, interview.FamilyId)
		ordered := make([]InterviewQuestion, 0, len(list))
		placed := false
		for _, row := range list {
			if row.Id == q.Id || row.OneOff {
				continue
			}
			ordered = append(ordered, row)
			if row.Id == after.Id {
				ordered = append(ordered, q)
				placed = true
			}
		}
		if !placed {
			ordered = append(ordered, q)
		}
		for i, row := range ordered {
			if row.Position != i || row.Id == q.Id {
				row.Position = i
				writeInterviewQuestion(tx, row)
			}
			if row.Id == q.Id {
				q = row
			}
		}
		listState := familyQuestionList(tx, interview.FamilyId)
		listState.Revision++
		writeQuestionList(tx, listState)
	} else {
		q, err = createOneOffQuestion(tx, interview.FamilyId, req.Text)
		if err != nil {
			return interview, q, err
		}
	}

	at := slices.Index(interview.QuestionIds, req.AfterQuestionId) + 1
	if at <= 0 {
		at = len(interview.QuestionIds)
	}
	interview.QuestionIds = slices.Insert(interview.QuestionIds, at, q.Id)
	writeInterview(tx, interview)
	return interview, q, nil
}

func AddInterviewQuestion(ctx *vbeam.Context, req AddInterviewQuestionRequest) (resp AddInterviewQuestionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	interview, person, err := interviewFor(ctx.Tx, user, req.InterviewId, AccessContribute)
	if err != nil {
		return
	}
	resp.Interview, resp.Question, err = addInterviewQuestionTx(ctx.Tx, interview, req)
	if err != nil {
		return
	}
	resp.Question.Text = renderQuestion(resp.Question.Text, person, interview.Date)
	vbolt.TxCommit(ctx.Tx)
	return
}

// Update and delete

type UpdateInterviewRequest struct {
	Id      int    `json:"id"`
	Version int    `json:"version"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Notes   string `json:"notes"`
	PhotoId int    `json:"photoId"`
	Status  string `json:"status"`
}

var ErrInterviewChanged = errors.New("Someone else changed this interview after you opened it. Reload to see their changes.")

func updateInterviewTx(tx *vbolt.Tx, interview Interview, person Person, req UpdateInterviewRequest) (Interview, error) {
	if req.Version != interview.Version {
		return interview, ErrInterviewChanged
	}
	title := strings.TrimSpace(req.Title)
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(title)) > maxInterviewTitle {
		return interview, errors.New("The title is too long")
	}
	if len([]rune(notes)) > maxInterviewNotes {
		return interview, errors.New("The notes are too long")
	}
	if req.Status != InterviewInProgress && req.Status != InterviewComplete {
		return interview, errors.New("Unknown interview status")
	}
	if req.Status == InterviewInProgress && interview.Status != InterviewInProgress {
		if other := inProgressInterview(tx, interview.PersonId); other.Id > 0 && other.Id != interview.Id {
			return interview, ErrInterviewInProgress
		}
	}
	if req.PhotoId > 0 && req.PhotoId != interview.PhotoId {
		if err := validatePhotoAccess(tx, req.PhotoId, interview.FamilyId); err != nil {
			return interview, err
		}
	}
	date, err := parseInterviewDate(req.Date, person.Birthday)
	if err != nil {
		return interview, err
	}

	if !date.Equal(interview.Date) {
		setInterviewDate(tx, &interview, date)
	}
	interview.Title = title
	interview.Notes = notes
	interview.PhotoId = max(req.PhotoId, 0)
	if req.Status == InterviewComplete && interview.Status != InterviewComplete {
		interview.CompletedAt = time.Now()
	}
	interview.Status = req.Status
	interview.Version++
	writeInterview(tx, interview)
	return interview, nil
}

func UpdateInterview(ctx *vbeam.Context, req UpdateInterviewRequest) (resp InterviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	interview, person, err := interviewFor(ctx.Tx, user, req.Id, AccessContribute)
	if err != nil {
		return
	}
	resp.Interview, err = updateInterviewTx(ctx.Tx, interview, person, req)
	if err != nil {
		return
	}
	vbolt.TxCommit(ctx.Tx)
	return
}

type DeleteInterviewRequest struct {
	Id int `json:"id"`
}

type DeleteInterviewResponse struct {
	PersonId int `json:"personId"`
}

func DeleteInterview(ctx *vbeam.Context, req DeleteInterviewRequest) (resp DeleteInterviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	interview, _, err := interviewFor(ctx.Tx, user, req.Id, AccessContribute)
	if err != nil {
		return
	}
	deleteInterviewTx(ctx.Tx, interview)
	resp.PersonId = interview.PersonId
	vbolt.TxCommit(ctx.Tx)
	return
}
