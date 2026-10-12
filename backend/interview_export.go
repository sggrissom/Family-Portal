package backend

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type ExportInterviewQuestion struct {
	Id          int    `json:"id"`
	Text        string `json:"text"`
	Section     string `json:"section"`
	Position    int    `json:"position"`
	MinAgeYears int    `json:"minAgeYears,omitempty"`
	MaxAgeYears int    `json:"maxAgeYears,omitempty"`
	DefaultKey  string `json:"defaultKey,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	OneOff      bool   `json:"oneOff,omitempty"`
}

type ExportInterview struct {
	Id          int       `json:"id"`
	PersonId    int       `json:"personId"`
	Date        time.Time `json:"date"`
	Title       string    `json:"title,omitempty"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	QuestionIds []int     `json:"questionIds"`
	SkippedIds  []int     `json:"skippedIds,omitempty"`
	PhotoId     int       `json:"photoId,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt"`
}

func buildInterviewExport(tx *vbolt.Tx, familyId int) ([]ExportInterviewQuestion, []ExportInterview) {
	questions := []ExportInterviewQuestion{}
	for _, q := range familyQuestions(tx, familyId) {
		questions = append(questions, ExportInterviewQuestion{
			Id: q.Id, Text: q.Text, Section: q.Section, Position: q.Position,
			MinAgeYears: q.MinAgeYears, MaxAgeYears: q.MaxAgeYears, DefaultKey: q.DefaultKey,
			Archived: q.Archived, OneOff: q.OneOff,
		})
	}
	interviews := []ExportInterview{}
	all := familyInterviews(tx, familyId)
	sortInterviewsByDate(all)
	for _, i := range all {
		interviews = append(interviews, ExportInterview{
			Id: i.Id, PersonId: i.PersonId, Date: i.Date, Title: i.Title, Source: i.Source, Status: i.Status,
			QuestionIds: i.QuestionIds, SkippedIds: i.SkippedIds, PhotoId: i.PhotoId, Notes: i.Notes,
			CreatedAt: i.CreatedAt, CompletedAt: i.CompletedAt,
		})
	}
	return questions, interviews
}

type interviewImport struct {
	familyId        int
	personIdMapping map[int]int
	tagNameToId     map[string]int
	photoIds        map[int]int
}

type interviewImportResult struct {
	imported     int
	skipped      int
	answers      int
	warnings     []string
	interviewIds map[int]int
	answerIds    []int
}

// mapQuestions finds or creates a family question for each exported one:
// built-ins by key, family questions by their text, one-offs always anew.
func (in interviewImport) mapQuestions(tx *vbolt.Tx, exported []ExportInterviewQuestion) map[int]int {
	ensureFamilyQuestions(tx, in.familyId)
	byKey := map[string]int{}
	byText := map[string]int{}
	position := 0
	for _, q := range familyQuestions(tx, in.familyId) {
		if q.DefaultKey != "" {
			byKey[q.DefaultKey] = q.Id
		}
		if !q.OneOff {
			byText[strings.ToLower(q.Text)] = q.Id
			position = max(position, q.Position+1)
		}
	}
	mapping := map[int]int{}
	created := false
	for _, eq := range exported {
		if id, ok := byKey[eq.DefaultKey]; ok && eq.DefaultKey != "" {
			mapping[eq.Id] = id
			continue
		}
		if id, ok := byText[strings.ToLower(eq.Text)]; ok && !eq.OneOff {
			mapping[eq.Id] = id
			continue
		}
		text, err := cleanQuestionText(eq.Text)
		if err != nil {
			continue
		}
		section, err := cleanQuestionSection(eq.Section)
		if err != nil {
			section = defaultSectionName
		}
		q := InterviewQuestion{
			FamilyId: in.familyId, Text: text, Section: section, OneOff: eq.OneOff, Archived: eq.Archived,
			MinAgeYears: eq.MinAgeYears, MaxAgeYears: eq.MaxAgeYears,
		}
		if validateQuestionAges(q.MinAgeYears, q.MaxAgeYears) != nil {
			q.MinAgeYears, q.MaxAgeYears = 0, 0
		}
		if q.OneOff {
			q.Section = oneOffSection
			q.Position = maxFamilyQuestions
		} else {
			q.Position = position
			position++
			created = true
		}
		q = createInterviewQuestion(tx, q)
		if !q.OneOff {
			byText[strings.ToLower(text)] = q.Id
		}
		mapping[eq.Id] = q.Id
	}
	if created {
		list := familyQuestionList(tx, in.familyId)
		list.Revision++
		writeQuestionList(tx, list)
	}
	return mapping
}

// importInterviews brings exported interviews and their answers across. An
// interview on a day the person already has one adds only the answers that
// interview doesn't have, so importing the same file twice changes nothing.
func (in interviewImport) run(tx *vbolt.Tx, questions []ExportInterviewQuestion, interviews []ExportInterview, milestones []ExportMilestone) interviewImportResult {
	result := interviewImportResult{interviewIds: map[int]int{}}
	if len(interviews) == 0 {
		return result
	}
	questionIds := in.mapQuestions(tx, questions)
	answersOf := map[int][]ExportMilestone{}
	for _, m := range milestones {
		if m.InterviewId > 0 {
			answersOf[m.InterviewId] = append(answersOf[m.InterviewId], m)
		}
	}

	for _, ei := range interviews {
		personId, ok := in.personIdMapping[ei.PersonId]
		if !ok {
			continue
		}
		person := GetPersonById(tx, personId)
		date := dayStart(ei.Date)
		if ei.Date.Year() <= 1 {
			result.warnings = append(result.warnings, fmt.Sprintf("Skipped an interview with no date for %s", person.Name))
			continue
		}

		interview := Interview{}
		for _, existing := range personInterviews(tx, personId) {
			if existing.Date.Equal(date) {
				interview = existing
				break
			}
		}
		isNew := interview.Id == 0
		if isNew {
			status := ei.Status
			if status != InterviewInProgress || inProgressInterview(tx, personId).Id > 0 {
				status = InterviewComplete
			}
			source := ei.Source
			if source != InterviewConducted {
				source = InterviewHistorical
			}
			interview = Interview{
				Id: vbolt.NextIntId(tx, InterviewBkt), FamilyId: person.FamilyId, PersonId: personId, Date: date,
				Title: ei.Title, Source: source, Status: status, QuestionIds: []int{}, SkippedIds: []int{},
				PhotoId: in.photoIds[ei.PhotoId], Notes: ei.Notes, CreatedAt: ei.CreatedAt,
				CompletedAt: ei.CompletedAt, Version: 1,
			}
			for _, id := range ei.QuestionIds {
				if q := questionIds[id]; q > 0 && !slices.Contains(interview.QuestionIds, q) {
					interview.QuestionIds = append(interview.QuestionIds, q)
				}
			}
			for _, id := range ei.SkippedIds {
				if q := questionIds[id]; q > 0 && !slices.Contains(interview.SkippedIds, q) {
					interview.SkippedIds = append(interview.SkippedIds, q)
				}
			}
		}

		answered := map[int]bool{}
		for _, m := range interviewAnswers(tx, interview.Id) {
			answered[m.QuestionId] = true
		}
		added := 0
		for _, em := range answersOf[ei.Id] {
			q := questionIds[em.QuestionId]
			text := strings.TrimSpace(em.Description)
			if q == 0 || answered[q] || text == "" {
				continue
			}
			answered[q] = true
			if !slices.Contains(interview.QuestionIds, q) {
				interview.QuestionIds = append(interview.QuestionIds, q)
			}
			m := Milestone{
				Id: vbolt.NextIntId(tx, MilestoneBkt), PersonId: personId, FamilyId: person.FamilyId,
				Description: text, Category: CategoryInterview, Context: em.Context, MilestoneDate: date,
				CreatedAt: time.Now(), InterviewId: interview.Id, QuestionId: q,
			}
			if m.Context == "" {
				m.Context = renderQuestion(GetInterviewQuestionById(tx, q).Text, person, date)
			}
			writeMilestone(tx, m)
			for _, tagName := range em.TagNames {
				if tagId, ok := in.tagNameToId[strings.ToLower(tagName)]; ok {
					addTagToMilestone(tx, m.Id, tagId, person.FamilyId)
				}
			}
			result.answerIds = append(result.answerIds, m.Id)
			added++
		}

		if isNew && (added > 0 || len(interview.QuestionIds) > 0) {
			writeInterview(tx, interview)
			result.imported++
		} else if !isNew && added > 0 {
			writeInterview(tx, interview)
			result.imported++
		} else {
			result.skipped++
		}
		if interview.Id > 0 && (!isNew || added > 0 || len(interview.QuestionIds) > 0) {
			result.interviewIds[ei.Id] = interview.Id
		}
		result.answers += added
	}
	return result
}
