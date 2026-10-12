package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterInterviewQuestionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetInterviewQuestions)
	vbeam.RegisterProc(app, SaveInterviewQuestions)
	vbeam.RegisterProc(app, MergeInterviewQuestions)
}

const (
	maxQuestionText     = 300
	maxQuestionSection  = 60
	maxFamilyQuestions  = 400
	maxQuestionAgeYears = 120
	defaultSectionName  = "More questions"
)

var ErrInterviewQuestionsChanged = errors.New("Someone else saved the question list after you opened it. Reload to see their changes.")
var ErrInterviewQuestionNotFound = errors.New("Question not found or not in your family")

type InterviewQuestion struct {
	Id          int       `json:"id"`
	FamilyId    int       `json:"familyId"`
	Text        string    `json:"text"`
	Section     string    `json:"section"`
	Position    int       `json:"position"`
	MinAgeYears int       `json:"minAgeYears"`
	MaxAgeYears int       `json:"maxAgeYears"`
	DefaultKey  string    `json:"defaultKey"`
	Archived    bool      `json:"archived"`
	OneOff      bool      `json:"oneOff"`
	CreatedAt   time.Time `json:"createdAt"`
}

func PackInterviewQuestion(self *InterviewQuestion, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Text, buf)
	vpack.String(&self.Section, buf)
	vpack.Int(&self.Position, buf)
	vpack.Int(&self.MinAgeYears, buf)
	vpack.Int(&self.MaxAgeYears, buf)
	vpack.String(&self.DefaultKey, buf)
	vpack.Bool(&self.Archived, buf)
	vpack.Bool(&self.OneOff, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// InterviewQuestionList holds the per-family state of the list itself.
// Seeded is set once the defaults have been copied into rows, and
// DismissedKeys are built-ins the family deleted, which are never offered again.
type InterviewQuestionList struct {
	FamilyId      int
	Revision      int
	Seeded        bool
	DismissedKeys []string
}

func PackInterviewQuestionList(self *InterviewQuestionList, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.Revision, buf)
	vpack.Bool(&self.Seeded, buf)
	vpack.Slice(&self.DismissedKeys, vpack.String, buf)
}

var InterviewQuestionBkt = vbolt.Bucket(&cfg.Info, "interview_questions", vpack.FInt, PackInterviewQuestion)
var InterviewQuestionByFamilyIndex = vbolt.Index(&cfg.Info, "interview_questions_by_family", vpack.FInt, vpack.FInt)
var InterviewQuestionListBkt = vbolt.Bucket(&cfg.Info, "interview_question_lists", vpack.FInt, PackInterviewQuestionList)

func GetInterviewQuestionById(tx *vbolt.Tx, id int) (question InterviewQuestion) {
	vbolt.Read(tx, InterviewQuestionBkt, id, &question)
	return
}

func familyQuestions(tx *vbolt.Tx, familyId int) (questions []InterviewQuestion) {
	var ids []int
	vbolt.ReadTermTargets(tx, InterviewQuestionByFamilyIndex, familyId, &ids, vbolt.Window{})
	if len(ids) > 0 {
		vbolt.ReadSlice(tx, InterviewQuestionBkt, ids, &questions)
	}
	sortQuestions(questions)
	return
}

func sortQuestions(questions []InterviewQuestion) {
	sort.SliceStable(questions, func(i, j int) bool {
		if questions[i].Position != questions[j].Position {
			return questions[i].Position < questions[j].Position
		}
		return questions[i].Id < questions[j].Id
	})
}

func familyQuestionList(tx *vbolt.Tx, familyId int) InterviewQuestionList {
	list := InterviewQuestionList{FamilyId: familyId}
	vbolt.Read(tx, InterviewQuestionListBkt, familyId, &list)
	return list
}

func writeQuestionList(tx *vbolt.Tx, list InterviewQuestionList) {
	vbolt.Write(tx, InterviewQuestionListBkt, list.FamilyId, &list)
}

func writeInterviewQuestion(tx *vbolt.Tx, question InterviewQuestion) {
	vbolt.Write(tx, InterviewQuestionBkt, question.Id, &question)
	vbolt.SetTargetSingleTerm(tx, InterviewQuestionByFamilyIndex, question.Id, question.FamilyId)
}

func deleteInterviewQuestion(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, InterviewQuestionBkt, id)
	vbolt.DeleteTargetTerms(tx, InterviewQuestionByFamilyIndex, id)
}

func createInterviewQuestion(tx *vbolt.Tx, question InterviewQuestion) InterviewQuestion {
	question.Id = vbolt.NextIntId(tx, InterviewQuestionBkt)
	question.CreatedAt = time.Now()
	writeInterviewQuestion(tx, question)
	return question
}

// ensureFamilyQuestions copies the built-in list into rows the first time a
// family writes anything interview-related. It is keyed on DefaultKey, so a
// second call adds nothing.
func ensureFamilyQuestions(tx *vbolt.Tx, familyId int) {
	list := familyQuestionList(tx, familyId)
	if list.Seeded {
		return
	}
	have := map[string]bool{}
	position := 0
	for _, q := range familyQuestions(tx, familyId) {
		have[q.DefaultKey] = true
		position = max(position, q.Position+1)
	}
	for _, d := range defaultInterviewQuestions {
		if have[d.Key] || slices.Contains(list.DismissedKeys, d.Key) {
			continue
		}
		row := d.row(familyId)
		row.Position = position
		position++
		createInterviewQuestion(tx, row)
	}
	list.Seeded = true
	writeQuestionList(tx, list)
}

type QuestionUsage struct {
	Interviews int `json:"interviews"`
	Answers    int `json:"answers"`
}

func questionUsage(tx *vbolt.Tx, familyId int, questions []InterviewQuestion) map[int]QuestionUsage {
	usage := map[int]QuestionUsage{}
	for _, q := range questions {
		if q.Id == 0 {
			continue
		}
		var answers int
		vbolt.ReadTermCount(tx, MilestoneByQuestionIndex, &q.Id, &answers)
		usage[q.Id] = QuestionUsage{Answers: answers}
	}
	for _, interview := range familyInterviews(tx, familyId) {
		for _, id := range interview.QuestionIds {
			if u, ok := usage[id]; ok {
				u.Interviews++
				usage[id] = u
			}
		}
	}
	return usage
}

func questionInUse(usage QuestionUsage) bool {
	return usage.Interviews > 0 || usage.Answers > 0
}

type GetInterviewQuestionsRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type InterviewQuestionsResponse struct {
	FamilyId  int                   `json:"familyId"`
	Revision  int                   `json:"revision"`
	Questions []InterviewQuestion   `json:"questions"`
	Suggested []InterviewQuestion   `json:"suggested"`
	Defaults  []InterviewQuestion   `json:"defaults"`
	Usage     map[int]QuestionUsage `json:"usage"`
}

func interviewQuestionsResponse(tx *vbolt.Tx, familyId int) InterviewQuestionsResponse {
	list := familyQuestionList(tx, familyId)
	resp := InterviewQuestionsResponse{
		FamilyId:  familyId,
		Revision:  list.Revision,
		Questions: []InterviewQuestion{},
		Suggested: []InterviewQuestion{},
		Defaults:  []InterviewQuestion{},
	}
	for i, d := range defaultInterviewQuestions {
		row := d.row(familyId)
		row.Position = i
		resp.Defaults = append(resp.Defaults, row)
	}

	if !list.Seeded {
		resp.Questions = append(resp.Questions, resp.Defaults...)
	}
	have := map[string]bool{}
	for _, q := range familyQuestions(tx, familyId) {
		have[q.DefaultKey] = true
		if !q.OneOff {
			resp.Questions = append(resp.Questions, q)
		}
	}
	if list.Seeded {
		for _, d := range resp.Defaults {
			if !have[d.DefaultKey] && !slices.Contains(list.DismissedKeys, d.DefaultKey) {
				resp.Suggested = append(resp.Suggested, d)
			}
		}
	}
	resp.Usage = questionUsage(tx, familyId, resp.Questions)
	return resp
}

func GetInterviewQuestions(ctx *vbeam.Context, req GetInterviewQuestionsRequest) (resp InterviewQuestionsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}
	resp = interviewQuestionsResponse(ctx.Tx, familyId)
	return
}

type InterviewQuestionInput struct {
	Id          int    `json:"id"`
	Text        string `json:"text"`
	Section     string `json:"section"`
	MinAgeYears int    `json:"minAgeYears"`
	MaxAgeYears int    `json:"maxAgeYears"`
	DefaultKey  string `json:"defaultKey"`
	Archived    bool   `json:"archived"`
}

type SaveInterviewQuestionsRequest struct {
	FamilyId  int                      `json:"familyId,omitempty"`
	Revision  int                      `json:"revision"`
	Questions []InterviewQuestionInput `json:"questions"`
	DeleteIds []int                    `json:"deleteIds"`
}

func cleanQuestionText(text string) (string, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "", errors.New("Every question needs some text")
	}
	if len([]rune(text)) > maxQuestionText {
		return "", fmt.Errorf("Questions must be %d characters or fewer", maxQuestionText)
	}
	return text, nil
}

func cleanQuestionSection(section string) (string, error) {
	section = strings.Join(strings.Fields(section), " ")
	if section == "" {
		return defaultSectionName, nil
	}
	if len([]rune(section)) > maxQuestionSection {
		return "", fmt.Errorf("Section names must be %d characters or fewer", maxQuestionSection)
	}
	return section, nil
}

func validateQuestionAges(minAge, maxAge int) error {
	if minAge < 0 || maxAge < 0 || minAge > maxQuestionAgeYears || maxAge > maxQuestionAgeYears {
		return errors.New("Ages must be between 0 and 120")
	}
	if maxAge > 0 && minAge > maxAge {
		return errors.New("A question's youngest age can't be above its oldest")
	}
	return nil
}

func applyQuestionInput(q *InterviewQuestion, in InterviewQuestionInput) error {
	text, err := cleanQuestionText(in.Text)
	if err != nil {
		return err
	}
	section, err := cleanQuestionSection(in.Section)
	if err != nil {
		return err
	}
	if err := validateQuestionAges(in.MinAgeYears, in.MaxAgeYears); err != nil {
		return err
	}
	q.Text = text
	q.Section = section
	q.MinAgeYears = in.MinAgeYears
	q.MaxAgeYears = in.MaxAgeYears
	q.Archived = in.Archived
	q.OneOff = false
	return nil
}

func saveInterviewQuestionsTx(tx *vbolt.Tx, familyId int, req SaveInterviewQuestionsRequest) error {
	list := familyQuestionList(tx, familyId)
	if req.Revision != list.Revision {
		return ErrInterviewQuestionsChanged
	}
	if len(req.Questions) > maxFamilyQuestions {
		return fmt.Errorf("A list can hold at most %d questions", maxFamilyQuestions)
	}

	for _, i := range familyInterviews(tx, familyId) {
		if i.Questions == nil {
			snapshotInterview(tx, &i)
			writeInterview(tx, i)
		}
	}
	existing := familyQuestions(tx, familyId)
	byId := map[int]InterviewQuestion{}
	byKey := map[string]InterviewQuestion{}
	for _, q := range existing {
		byId[q.Id] = q
		if q.DefaultKey != "" {
			byKey[q.DefaultKey] = q
		}
	}

	if !list.Seeded {
		sent := map[string]bool{}
		for _, in := range req.Questions {
			sent[in.DefaultKey] = true
		}
		for _, d := range defaultInterviewQuestions {
			if !sent[d.Key] && byKey[d.Key].Id == 0 && !slices.Contains(list.DismissedKeys, d.Key) {
				list.DismissedKeys = append(list.DismissedKeys, d.Key)
			}
		}
		list.Seeded = true
	}

	usage := questionUsage(tx, familyId, existing)
	for _, id := range req.DeleteIds {
		q, ok := byId[id]
		if !ok {
			return ErrInterviewQuestionNotFound
		}
		if questionInUse(usage[id]) {
			return fmt.Errorf("%q has been asked before, so it can be retired but not deleted", q.Text)
		}
	}

	seen := map[int]bool{}
	position := 0
	for _, in := range req.Questions {
		var q InterviewQuestion
		switch {
		case in.Id > 0:
			found, ok := byId[in.Id]
			if !ok || slices.Contains(req.DeleteIds, in.Id) {
				return ErrInterviewQuestionNotFound
			}
			q = found
		case in.DefaultKey != "" && byKey[in.DefaultKey].Id > 0:
			q = byKey[in.DefaultKey]
		default:
			q = InterviewQuestion{FamilyId: familyId}
			if _, known := defaultInterviewQuestionByKey(in.DefaultKey); known {
				q.DefaultKey = in.DefaultKey
			}
		}
		if q.Id > 0 && seen[q.Id] {
			return errors.New("A question appears twice in the list")
		}
		if err := applyQuestionInput(&q, in); err != nil {
			return err
		}
		q.Position = position
		position++
		if q.Id == 0 {
			q = createInterviewQuestion(tx, q)
		} else {
			writeInterviewQuestion(tx, q)
		}
		seen[q.Id] = true
		if q.DefaultKey != "" {
			byKey[q.DefaultKey] = q
			list.DismissedKeys = slices.DeleteFunc(list.DismissedKeys, func(k string) bool { return k == q.DefaultKey })
		}
	}

	for _, q := range existing {
		if seen[q.Id] || q.OneOff || slices.Contains(req.DeleteIds, q.Id) {
			continue
		}
		q.Position = position
		position++
		writeInterviewQuestion(tx, q)
	}

	for _, id := range req.DeleteIds {
		q := byId[id]
		deleteInterviewQuestion(tx, id)
		if q.DefaultKey != "" && !slices.Contains(list.DismissedKeys, q.DefaultKey) {
			list.DismissedKeys = append(list.DismissedKeys, q.DefaultKey)
		}
	}

	list.Revision++
	writeQuestionList(tx, list)
	return nil
}

func SaveInterviewQuestions(ctx *vbeam.Context, req SaveInterviewQuestionsRequest) (resp InterviewQuestionsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessContribute)
	if err != nil {
		return
	}
	vbeam.UseWriteTx(ctx)
	if err = saveInterviewQuestionsTx(ctx.Tx, familyId, req); err != nil {
		return
	}
	resp = interviewQuestionsResponse(ctx.Tx, familyId)
	vbolt.TxCommit(ctx.Tx)
	return
}

type MergeInterviewQuestionsRequest struct {
	FromId   int `json:"fromId"`
	IntoId   int `json:"intoId"`
	Revision int `json:"revision"`
}

func mergeInterviewQuestionsTx(tx *vbolt.Tx, user User, req MergeInterviewQuestionsRequest) (familyId int, err error) {
	from := GetInterviewQuestionById(tx, req.FromId)
	into := GetInterviewQuestionById(tx, req.IntoId)
	if from.Id == 0 || into.Id == 0 || from.FamilyId != into.FamilyId ||
		!CanAccessFamily(tx, user, from.FamilyId, AccessContribute) {
		return 0, ErrInterviewQuestionNotFound
	}
	if from.Id == into.Id {
		return 0, errors.New("Pick a different question to merge into")
	}
	familyId = from.FamilyId
	list := familyQuestionList(tx, familyId)
	if req.Revision != list.Revision {
		return 0, ErrInterviewQuestionsChanged
	}

	for _, i := range familyInterviews(tx, familyId) {
		if slices.Contains(i.QuestionIds, from.Id) && slices.Contains(i.QuestionIds, into.Id) {
			return 0, errors.New("Both questions are planned in the same interview, so they can't be merged")
		}
	}
	answers := questionAnswers(tx, from.Id)
	answeredInto := map[int]bool{}
	for _, m := range questionAnswers(tx, into.Id) {
		answeredInto[m.InterviewId] = true
	}
	for _, m := range answers {
		if answeredInto[m.InterviewId] {
			return 0, fmt.Errorf("Both questions were answered in the same interview, so they can't be merged")
		}
	}
	for _, m := range answers {
		m.QuestionId = into.Id
		writeMilestone(tx, m)
		answeredInto[m.InterviewId] = true
	}
	for _, interview := range familyInterviews(tx, familyId) {
		if !slices.Contains(interview.QuestionIds, from.Id) && !slices.Contains(interview.SkippedIds, from.Id) {
			continue
		}
		snapshotInterview(tx, &interview)
		for j := range interview.Questions {
			if interview.Questions[j].QuestionId == from.Id {
				interview.Questions[j].QuestionId = into.Id
			}
		}
		interview.Version++
		interview.QuestionIds = replaceId(interview.QuestionIds, from.Id, into.Id)
		interview.SkippedIds = replaceId(interview.SkippedIds, from.Id, into.Id)
		if answeredInto[interview.Id] {
			interview.SkippedIds = slices.DeleteFunc(interview.SkippedIds, func(id int) bool { return id == into.Id })
		}
		writeInterview(tx, interview)
	}

	from.Archived = true
	writeInterviewQuestion(tx, from)
	list.Revision++
	writeQuestionList(tx, list)
	return familyId, nil
}

func MergeInterviewQuestions(ctx *vbeam.Context, req MergeInterviewQuestionsRequest) (resp InterviewQuestionsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	familyId, err := mergeInterviewQuestionsTx(ctx.Tx, user, req)
	if err != nil {
		return
	}
	resp = interviewQuestionsResponse(ctx.Tx, familyId)
	vbolt.TxCommit(ctx.Tx)
	return
}
