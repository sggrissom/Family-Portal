package backend

import (
	"crypto/sha256"
	"encoding/json"
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
	Id          int                         `json:"id"`
	PersonId    int                         `json:"personId"`
	Date        time.Time                   `json:"date"`
	Title       string                      `json:"title,omitempty"`
	Source      string                      `json:"source"`
	Status      string                      `json:"status"`
	QuestionIds []int                       `json:"questionIds"`
	SkippedIds  []int                       `json:"skippedIds,omitempty"`
	PhotoId     int                         `json:"photoId,omitempty"`
	Notes       string                      `json:"notes,omitempty"`
	CreatedAt   time.Time                   `json:"createdAt"`
	CompletedAt time.Time                   `json:"completedAt"`
	Questions   []InterviewQuestionSnapshot `json:"questions,omitempty"`
	OriginKey   string                      `json:"originKey,omitempty"`
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
		snapshotInterview(tx, &i)
		interviews = append(interviews, ExportInterview{
			Id: i.Id, PersonId: i.PersonId, Date: i.Date, Title: i.Title, Source: i.Source, Status: i.Status,
			QuestionIds: i.QuestionIds, SkippedIds: i.SkippedIds, PhotoId: i.PhotoId, Notes: i.Notes,
			CreatedAt: i.CreatedAt, CompletedAt: i.CompletedAt, Questions: i.Questions, OriginKey: interviewOrigin(i),
		})
	}
	return questions, interviews
}

type interviewImport struct {
	familyId        int
	personIdMapping map[int]int
	tagNameToId     map[string]int
	photoIds        map[int]int
	list            *InterviewQuestionList
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
	state := familyQuestionList(tx, in.familyId)
	restoring := in.list != nil && !state.Seeded && len(familyQuestions(tx, in.familyId)) == 0
	if restoring {
		state = *in.list
		state.FamilyId = in.familyId
		writeQuestionList(tx, state)
	} else {
		ensureFamilyQuestions(tx, in.familyId)
	}
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
			MinAgeYears: eq.MinAgeYears, MaxAgeYears: eq.MaxAgeYears, DefaultKey: eq.DefaultKey,
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

// Stable source identity preserves distinct same-day sittings and makes bundle replay safe.
func (in interviewImport) run(tx *vbolt.Tx, questions []ExportInterviewQuestion, interviews []ExportInterview, milestones []ExportMilestone) interviewImportResult {
	result := interviewImportResult{interviewIds: map[int]int{}}
	answersOf := map[int][]ExportMilestone{}
	for _, m := range milestones {
		if m.InterviewId > 0 {
			answersOf[m.InterviewId] = append(answersOf[m.InterviewId], m)
		}
	}

	// Replays preserve the entire existing sitting, including edits since export.
	// Filter before mapping one-offs so replay cannot create orphan questions.
	pending := []ExportInterview{}
	needed := map[int]bool{}
	for _, ei := range interviews {
		if ei.OriginKey == "" {
			bytes, _ := json.Marshal(struct {
				Interview ExportInterview
				Answers   []ExportMilestone
			}{ei, answersOf[ei.Id]})
			ei.OriginKey = fmt.Sprintf("bundle:%x", sha256.Sum256(bytes))
		}
		found := false
		for _, existing := range personInterviews(tx, in.personIdMapping[ei.PersonId]) {
			if interviewOrigin(existing) == ei.OriginKey {
				result.interviewIds[ei.Id] = existing.Id
				result.skipped++
				result.warnings = append(result.warnings, fmt.Sprintf("Kept existing interview %d from this source; no answers were overwritten", existing.Id))
				found = true
				break
			}
		}
		if !found {
			pending = append(pending, ei)
			for _, id := range ei.QuestionIds {
				needed[id] = true
			}
		}
	}
	mappedQuestions := []ExportInterviewQuestion{}
	for _, q := range questions {
		if !q.OneOff || needed[q.Id] {
			mappedQuestions = append(mappedQuestions, q)
		}
	}
	questionIds := in.mapQuestions(tx, mappedQuestions)
	for _, ei := range pending {
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

		origin := ei.OriginKey
		if origin == "" {
			bytes, _ := json.Marshal(struct {
				Interview ExportInterview
				Answers   []ExportMilestone
			}{ei, answersOf[ei.Id]})
			origin = fmt.Sprintf("bundle:%x", sha256.Sum256(bytes))
		}
		interview := Interview{}
		for _, existing := range personInterviews(tx, personId) {
			if interviewOrigin(existing) == origin {
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
				CompletedAt: ei.CompletedAt, Version: 1, OriginKey: origin,
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

		if isNew {
			for _, snap := range ei.Questions {
				if q := questionIds[snap.QuestionId]; q > 0 {
					snap.QuestionId = q
					interview.Questions = append(interview.Questions, snap)
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
			interview.SkippedIds = slices.DeleteFunc(interview.SkippedIds, func(id int) bool { return id == q })
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
			photos := []int{}
			for _, old := range em.PhotoIds {
				if mapped := in.photoIds[old]; mapped > 0 {
					photos = append(photos, mapped)
				}
			}
			if err := setMilestonePhotos(tx, m, photos); err != nil {
				result.warnings = append(result.warnings, err.Error())
			}
			for _, tagName := range em.TagNames {
				if tagId, ok := in.tagNameToId[strings.ToLower(tagName)]; ok {
					addTagToMilestone(tx, m.Id, tagId, person.FamilyId)
				}
			}
			result.answerIds = append(result.answerIds, m.Id)
			added++
		}

		if isNew && (added > 0 || len(interview.QuestionIds) > 0) {
			snapshotInterview(tx, &interview)
			writeInterview(tx, interview)
			result.imported++
		} else if !isNew && added > 0 {
			interview.Version++
			snapshotInterview(tx, &interview)
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

func validateInterviewBundle(data ImportDataStructure) error {
	questions := map[int]bool{}
	for _, q := range data.InterviewQuestions {
		if q.Id <= 0 || questions[q.Id] {
			return fmt.Errorf("Invalid or duplicate interview question id %d", q.Id)
		}
		if _, err := cleanQuestionText(q.Text); err != nil {
			return err
		}
		questions[q.Id] = true
	}
	interviews := map[int]ExportInterview{}
	for _, i := range data.Interviews {
		if i.Id <= 0 || interviews[i.Id].Id > 0 {
			return fmt.Errorf("Invalid or duplicate interview id %d", i.Id)
		}
		planned := map[int]bool{}
		for _, id := range i.QuestionIds {
			if !questions[id] || planned[id] {
				return fmt.Errorf("Interview %d has an invalid question reference", i.Id)
			}
			planned[id] = true
		}
		for _, id := range i.SkippedIds {
			if !planned[id] {
				return fmt.Errorf("Interview %d skips an unplanned question", i.Id)
			}
		}
		seen := map[int]bool{}
		for _, q := range i.Questions {
			if !planned[q.QuestionId] || seen[q.QuestionId] {
				return fmt.Errorf("Interview %d has invalid prompt snapshots", i.Id)
			}
			if _, err := cleanQuestionText(q.Text); err != nil {
				return err
			}
			seen[q.QuestionId] = true
		}
		if i.Questions != nil && len(i.Questions) != len(i.QuestionIds) {
			return fmt.Errorf("Interview %d has incomplete prompt snapshots", i.Id)
		}
		interviews[i.Id] = i
	}
	answers := map[string]bool{}
	for _, m := range data.Milestones {
		if m.InterviewId == 0 && m.Category != CategoryInterview {
			continue
		}
		i := interviews[m.InterviewId]
		key := fmt.Sprintf("%d:%d", m.InterviewId, m.QuestionId)
		if i.Id == 0 || m.PersonId != i.PersonId || !slices.Contains(i.QuestionIds, m.QuestionId) || slices.Contains(i.SkippedIds, m.QuestionId) || answers[key] {
			return fmt.Errorf("Moment %d has invalid interview linkage", m.Id)
		}
		answers[key] = true
	}
	return nil
}
