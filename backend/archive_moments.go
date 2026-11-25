package backend

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type ExportMilestone struct {
	Id            int         `json:"id"`
	PersonId      int         `json:"personId"`
	FamilyId      int         `json:"familyId"`
	Description   string      `json:"description"`
	Category      string      `json:"category"`
	Context       string      `json:"context,omitempty"`
	MilestoneDate time.Time   `json:"milestoneDate"`
	RecordTime    *RecordTime `json:"recordTime,omitempty"`
	DateLabel     string      `json:"dateLabel,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	PersonName    string      `json:"personName"`
	TagNames      []string    `json:"tagNames,omitempty"`
	PhotoIds      []int       `json:"photoIds,omitempty"`
	// TagsAddedAt and PhotosAddedAt are when each tag, by lowercased name,
	// and each photo, by PhotoIds id, was attached.
	TagsAddedAt   map[string]time.Time `json:"tagsAddedAt,omitempty"`
	PhotosAddedAt map[int]time.Time    `json:"photosAddedAt,omitempty"`
	InterviewId   int                  `json:"interviewId,omitempty"`
	QuestionId    int                  `json:"questionId,omitempty"`
	Note          string               `json:"note,omitempty"`
	RecordedBy    string               `json:"recordedBy,omitempty"`
}

func buildMilestoneExport(tx *vbolt.Tx, familyId int, people []Person, tagIdToName map[int]string, recorderOf func(userId int) string) []ExportMilestone {
	var milestoneIds []int
	vbolt.ReadTermTargets(tx, MilestoneByFamilyIndex, familyId, &milestoneIds, vbolt.Window{})

	var milestones []Milestone
	if len(milestoneIds) > 0 {
		vbolt.ReadSlice(tx, MilestoneBkt, milestoneIds, &milestones)
	}

	exportMilestones := make([]ExportMilestone, len(milestones))
	for i, milestone := range milestones {
		var personName string
		for _, person := range people {
			if person.Id == milestone.PersonId {
				personName = person.Name
				break
			}
		}

		var tagNames []string
		tagsAddedAt := map[string]time.Time{}
		for _, link := range milestoneTagLinks(tx, MilestoneTagByMilestoneIndex, milestone.Id) {
			if name, ok := tagIdToName[link.TagId]; ok {
				tagNames = append(tagNames, name)
				tagsAddedAt[strings.ToLower(name)] = link.CreatedAt
			}
		}
		var photoIds []int
		photosAddedAt := map[int]time.Time{}
		for _, link := range milestonePhotoLinks(tx, MilestonePhotoByMilestoneIndex, milestone.Id) {
			photoIds = append(photoIds, link.PhotoId)
			photosAddedAt[link.PhotoId] = link.CreatedAt
		}

		exportMilestones[i] = ExportMilestone{
			Id:            milestone.Id,
			PersonId:      milestone.PersonId,
			FamilyId:      milestone.FamilyId,
			Description:   milestone.Description,
			Category:      milestone.Category,
			Context:       milestone.Context,
			MilestoneDate: milestone.MilestoneDate,
			RecordTime:    &milestone.RecordTime,
			DateLabel:     milestone.RecordTime.Label(),
			CreatedAt:     milestone.CreatedAt,
			PersonName:    personName,
			TagNames:      tagNames,
			PhotoIds:      photoIds,
			TagsAddedAt:   tagsAddedAt,
			PhotosAddedAt: photosAddedAt,
			InterviewId:   milestone.InterviewId,
			QuestionId:    milestone.QuestionId,
			Note:          milestone.Note,
			RecordedBy:    recorderOf(milestone.CreatedBy),
		}
	}
	return exportMilestones
}

func filterMilestones(milestones []ExportMilestone, personIdMapping map[int]int) []ExportMilestone {
	var filteredMilestones []ExportMilestone

	for _, milestone := range milestones {
		if _, exists := personIdMapping[milestone.PersonId]; exists {
			filteredMilestones = append(filteredMilestones, milestone)
		}
	}

	return filteredMilestones
}

func existingMilestoneId(tx *vbolt.Tx, personId int, date time.Time, description string) int {
	for _, milestone := range GetPersonMilestonesTx(tx, personId) {
		if milestone.MilestoneDate.Equal(date) && milestone.Description == description {
			return milestone.Id
		}
	}
	return 0
}

func importMilestones(tx *vbolt.Tx, importMilestones []ExportMilestone, familyId int, l *archiveLedger) (int, int, []string, []int) {
	var ids []int
	var errors []string
	importedCount := 0
	skippedCount := 0
	recorderOf := importRecorders(tx, familyId)

	for _, milestone := range importMilestones {
		if milestone.InterviewId > 0 || milestone.Category == CategoryInterview {
			continue
		}
		newPersonId, exists := l.people[milestone.PersonId]
		if !exists {
			errors = append(errors, fmt.Sprintf("Moment for unknown person ID: %d", milestone.PersonId))
			continue
		}

		if milestone.RecordTime == nil {
			if milestone.MilestoneDate.Year() == 1 {
				continue
			}
			milestone.MilestoneDate = dayStart(milestone.MilestoneDate)
		}
		recordTime, date, err := importRecordTime(milestone.RecordTime, milestone.MilestoneDate, l.people, true)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Moment %q: %s", milestone.Description, err.Error()))
			continue
		}
		milestone.MilestoneDate = date

		if id := existingMilestoneId(tx, newPersonId, milestone.MilestoneDate, milestone.Description); id != 0 {
			l.moments[milestone.Id] = id
			skippedCount++
			continue
		}

		newMilestone := Milestone{
			Id:            vbolt.NextIntId(tx, MilestoneBkt),
			PersonId:      newPersonId,
			FamilyId:      familyId,
			Description:   milestone.Description,
			Category:      milestone.Category,
			Context:       milestone.Context,
			MilestoneDate: milestone.MilestoneDate,
			RecordTime:    recordTime,
			CreatedAt:     archiveCreatedAt(milestone.CreatedAt),
			CreatedBy:     recorderOf(milestone.RecordedBy),
			Note:          cleanAnswerNote(milestone.Note),
		}
		writeMilestone(tx, &newMilestone)
		l.moments[milestone.Id] = newMilestone.Id

		for _, tagName := range milestone.TagNames {
			if tagId, ok := l.tagByName[strings.ToLower(tagName)]; ok {
				linkTagToMilestone(tx, newMilestone.Id, tagId, familyId, archiveCreatedAt(milestone.TagsAddedAt[strings.ToLower(tagName)]))
			}
		}

		ids = append(ids, newMilestone.Id)
		importedCount++
	}

	return importedCount, skippedCount, errors, ids
}

// archiveCreatedAt keeps when a record was added, or stamps it now when the
// archive predates carrying it.
func archiveCreatedAt(at time.Time) time.Time {
	if at.IsZero() {
		return time.Now()
	}
	return at
}

// linkMomentPhotos reattaches each moment's photos once both are imported.
// A link to a photo the archive didn't carry is left out.
func linkMomentPhotos(tx *vbolt.Tx, milestones []ExportMilestone, familyId int, l *archiveLedger) {
	for _, m := range milestones {
		momentId, ok := l.moments[m.Id]
		if !ok {
			continue
		}
		for _, photoId := range m.PhotoIds {
			if id, ok := l.photos[photoId]; ok {
				linkPhotoToMilestone(tx, momentId, id, familyId, archiveCreatedAt(m.PhotosAddedAt[photoId]))
			}
		}
	}
}

// mapExistingInterviews links the interviews a family already has, so books
// can point at them when the import leaves moments out.
func mapExistingInterviews(tx *vbolt.Tx, interviews []ExportInterview, l *archiveLedger) {
	for _, ei := range interviews {
		personId, ok := l.people[ei.PersonId]
		if !ok {
			continue
		}
		for _, existing := range personInterviews(tx, personId) {
			if existing.Date.Equal(dayStart(ei.Date)) {
				l.interviews[ei.Id] = existing.Id
				break
			}
		}
	}
}

// mapExistingMoments links the moments a family already has, so books can
// point at them when the import leaves moments out.
func mapExistingMoments(tx *vbolt.Tx, milestones []ExportMilestone, l *archiveLedger) {
	for _, m := range milestones {
		if personId, ok := l.people[m.PersonId]; ok {
			if id := existingMilestoneId(tx, personId, m.MilestoneDate, m.Description); id != 0 {
				l.moments[m.Id] = id
			}
		}
	}
}

type ExportInterviewQuestion struct {
	Id          int       `json:"id"`
	Text        string    `json:"text"`
	Section     string    `json:"section"`
	Position    int       `json:"position"`
	MinAgeYears int       `json:"minAgeYears,omitempty"`
	MaxAgeYears int       `json:"maxAgeYears,omitempty"`
	DefaultKey  string    `json:"defaultKey,omitempty"`
	Archived    bool      `json:"archived,omitempty"`
	OneOff      bool      `json:"oneOff,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
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
			Archived: q.Archived, OneOff: q.OneOff, CreatedAt: q.CreatedAt,
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
	familyId int
	ledger   *archiveLedger
}

type interviewImportResult struct {
	imported  int
	skipped   int
	answers   int
	warnings  []string
	answerIds []int
}

// mapQuestions finds or creates a family question for each exported one:
// built-ins by key, family questions by their text, one-offs always anew.
func (in interviewImport) mapQuestions(tx *vbolt.Tx, exported []ExportInterviewQuestion) map[int]int {
	seededHere := !familyQuestionList(tx, in.familyId).Seeded
	ensureFamilyQuestions(tx, in.familyId)
	byKey := map[string]int{}
	byText := map[string]int{}
	rows := map[int]InterviewQuestion{}
	position := 0
	for _, q := range familyQuestions(tx, in.familyId) {
		rows[q.Id] = q
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
			if q := rows[id]; seededHere && !eq.CreatedAt.IsZero() && !q.CreatedAt.Equal(eq.CreatedAt) {
				q.CreatedAt = eq.CreatedAt
				writeInterviewQuestion(tx, q)
			}
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
		if !eq.CreatedAt.IsZero() {
			q.CreatedAt = eq.CreatedAt
			writeInterviewQuestion(tx, q)
		}
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
	var result interviewImportResult
	if len(interviews) == 0 {
		return result
	}
	questionIds := in.mapQuestions(tx, questions)
	recorderOf := importRecorders(tx, in.familyId)
	answersOf := map[int][]ExportMilestone{}
	for _, m := range milestones {
		if m.InterviewId > 0 {
			answersOf[m.InterviewId] = append(answersOf[m.InterviewId], m)
		}
	}

	for _, ei := range interviews {
		personId, ok := in.ledger.people[ei.PersonId]
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
				PhotoId: in.ledger.photos[ei.PhotoId], Notes: ei.Notes, CreatedAt: ei.CreatedAt,
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

		answered := map[int]int{}
		for _, m := range interviewAnswers(tx, interview.Id) {
			answered[m.QuestionId] = m.Id
		}
		added := 0
		for _, em := range answersOf[ei.Id] {
			q := questionIds[em.QuestionId]
			text := strings.TrimSpace(em.Description)
			if id := answered[q]; id != 0 {
				in.ledger.moments[em.Id] = id
			}
			if q == 0 || answered[q] != 0 || text == "" {
				continue
			}
			if !slices.Contains(interview.QuestionIds, q) {
				interview.QuestionIds = append(interview.QuestionIds, q)
			}
			m := Milestone{
				Id: vbolt.NextIntId(tx, MilestoneBkt), PersonId: personId, FamilyId: person.FamilyId,
				Description: text, Category: CategoryInterview, Context: em.Context, MilestoneDate: date,
				CreatedAt: archiveCreatedAt(em.CreatedAt), CreatedBy: recorderOf(em.RecordedBy), InterviewId: interview.Id, QuestionId: q,
				Note: cleanAnswerNote(em.Note),
			}
			if m.Context == "" {
				m.Context = renderQuestion(GetInterviewQuestionById(tx, q).Text, person, date)
			}
			writeMilestone(tx, &m)
			answered[q] = m.Id
			in.ledger.moments[em.Id] = m.Id
			for _, tagName := range em.TagNames {
				if tagId, ok := in.ledger.tagByName[strings.ToLower(tagName)]; ok {
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
			in.ledger.interviews[ei.Id] = interview.Id
		}
		result.answers += added
	}
	return result
}

type ExportBookItem struct {
	Kind     string `json:"kind"`
	SourceId int    `json:"sourceId"`
	PhotoId  int    `json:"photoId,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Pinned   bool   `json:"pinned,omitempty"`
}

type ExportBook struct {
	Id           int              `json:"id"`
	PersonId     int              `json:"personId"`
	PersonIds    []int            `json:"personIds,omitempty"`
	Preset       string           `json:"preset"`
	Title        string           `json:"title"`
	StartDate    time.Time        `json:"startDate"`
	EndDate      time.Time        `json:"endDate"`
	CoverPhotoId int              `json:"coverPhotoId,omitempty"`
	Density      string           `json:"density"`
	Categories   []string         `json:"categories,omitempty"`
	Match        string           `json:"match,omitempty"`
	Introduction string           `json:"introduction,omitempty"`
	Letter       string           `json:"letter,omitempty"`
	Signature    string           `json:"signature,omitempty"`
	ShowGrowth   bool             `json:"showGrowth"`
	Items        []ExportBookItem `json:"items"`
	Excluded     []ExportBookItem `json:"excluded,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
	ReviewedAt   time.Time        `json:"reviewedAt"`
}

var bookItemKindNames = map[BookItemKind]string{
	BookItemMilestone: "milestone",
	BookItemPhoto:     "photo",
	BookItemInterview: "interview",
}

func exportBookItems(items []BookItem) []ExportBookItem {
	out := make([]ExportBookItem, 0, len(items))
	for _, item := range items {
		out = append(out, ExportBookItem{
			Kind:     bookItemKindNames[item.Kind],
			SourceId: item.SourceId,
			PhotoId:  item.PhotoId,
			Caption:  item.Caption,
			Pinned:   item.Pinned,
		})
	}
	return out
}

func buildBookExport(tx *vbolt.Tx, familyId int) []ExportBook {
	books := []ExportBook{}
	for _, book := range GetFamilyBooks(tx, familyId) {
		books = append(books, ExportBook{
			Id:           book.Id,
			PersonId:     book.PersonId,
			PersonIds:    book.PersonIds,
			Preset:       book.Preset,
			Title:        book.Title,
			StartDate:    book.StartDate,
			EndDate:      book.EndDate,
			CoverPhotoId: book.CoverPhotoId,
			Density:      book.Density,
			Categories:   book.Categories,
			Match:        book.Match,
			Introduction: book.Introduction,
			Letter:       book.Letter,
			Signature:    book.Signature,
			ShowGrowth:   book.ShowGrowth,
			Items:        exportBookItems(book.Items),
			Excluded:     exportBookItems(book.Excluded),
			CreatedAt:    book.CreatedAt,
			UpdatedAt:    book.UpdatedAt,
			ReviewedAt:   book.ReviewedAt,
		})
	}
	return books
}

type bookImport struct {
	familyId int
	userId   int
	ledger   *archiveLedger
}

func (in bookImport) mapItem(item ExportBookItem) (BookItem, bool) {
	out := BookItem{SourceId: 0, Caption: item.Caption, Pinned: item.Pinned}
	var ok bool
	switch item.Kind {
	case "milestone":
		out.Kind = BookItemMilestone
		out.SourceId, ok = in.ledger.moments[item.SourceId]
		if item.PhotoId != 0 {
			out.PhotoId = in.ledger.photos[item.PhotoId]
		}
	case "photo":
		out.Kind = BookItemPhoto
		out.SourceId, ok = in.ledger.photos[item.SourceId]
	case "interview":
		out.Kind = BookItemInterview
		out.SourceId, ok = in.ledger.interviews[item.SourceId]
	}
	return out, ok && out.SourceId != 0
}

func (in bookImport) mapPeople(source ExportBook) []int {
	ids := source.PersonIds
	if len(ids) == 0 {
		ids = []int{source.PersonId}
	}
	mapped := []int{}
	seen := map[int]bool{}
	for _, id := range ids {
		if newId, ok := in.ledger.people[id]; ok && !seen[newId] {
			seen[newId] = true
			mapped = append(mapped, newId)
		}
	}
	return mapped
}

func bookAlreadyImported(tx *vbolt.Tx, personId int, source ExportBook) bool {
	for _, existing := range GetPersonBooks(tx, personId) {
		if existing.Preset == source.Preset && existing.Title == source.Title && existing.StartDate.Equal(source.StartDate) {
			return true
		}
	}
	return false
}

func importBooks(tx *vbolt.Tx, books []ExportBook, in bookImport) (imported, skipped int, warnings []string) {
	now := time.Now()
	for _, source := range books {
		personIds := in.mapPeople(source)
		if len(personIds) == 0 || bookAlreadyImported(tx, personIds[0], source) {
			skipped++
			continue
		}

		book := Book{
			Id:           vbolt.NextIntId(tx, BookBkt),
			FamilyId:     in.familyId,
			PersonIds:    personIds,
			Preset:       source.Preset,
			Title:        source.Title,
			StartDate:    source.StartDate,
			EndDate:      source.EndDate,
			CoverPhotoId: in.ledger.photos[source.CoverPhotoId],
			Density:      cleanDensity(source.Density),
			Categories:   cleanCategories(source.Categories),
			Match:        cleanMatch(source.Match),
			Introduction: source.Introduction,
			Letter:       source.Letter,
			Signature:    source.Signature,
			ShowGrowth:   source.ShowGrowth,
			Items:        []BookItem{},
			Excluded:     []BookItem{},
			Revision:     1,
			CreatedBy:    in.userId,
			CreatedAt:    now,
			UpdatedAt:    now,
			ReviewedAt:   now,
		}
		if !source.CreatedAt.IsZero() {
			book.CreatedAt, book.UpdatedAt, book.ReviewedAt = source.CreatedAt, source.UpdatedAt, source.ReviewedAt
		}
		if book.Preset == "" {
			book.Preset = BookPresetFirstYear
		}

		unmatched := 0
		for _, item := range source.Items {
			mapped, ok := in.mapItem(item)
			if !ok {
				unmatched++
				continue
			}
			if item.PhotoId != 0 && mapped.PhotoId == 0 {
				unmatched++
			}
			book.Items = append(book.Items, mapped)
		}
		for _, item := range source.Excluded {
			if mapped, ok := in.mapItem(item); ok {
				book.Excluded = append(book.Excluded, BookItem{Kind: mapped.Kind, SourceId: mapped.SourceId})
			}
		}
		if source.CoverPhotoId != 0 && book.CoverPhotoId == 0 {
			unmatched++
		}

		writeBookTx(tx, book)
		imported++
		if unmatched > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"Book %q: %d of its photos or records were not in this import and were left out", book.Title, unmatched))
		}
	}
	return
}

func exportInterviewCadence(tx *vbolt.Tx, familyId int) *InterviewCadence {
	list := familyQuestionList(tx, familyId)
	if list.Cadence == "" {
		return nil
	}
	cadence := list.cadence()
	return &cadence
}

// importInterviewListSettings takes the exported cadence only when the family
// hasn't chosen one of its own, and dismisses the built-ins the archive
// dismissed unless the family still has them.
func importInterviewListSettings(tx *vbolt.Tx, familyId int, exported *InterviewCadence, dismissed []string) {
	list := familyQuestionList(tx, familyId)
	changed := false
	if exported != nil {
		if cadence, err := validCadence(*exported); err == nil && list.Cadence == "" {
			list.Cadence = cadence.Kind
			list.CadenceMonth = cadence.Month
			changed = true
		}
	}
	have := map[string]bool{}
	for _, q := range familyQuestions(tx, familyId) {
		have[q.DefaultKey] = true
	}
	for _, key := range dismissed {
		if _, known := defaultInterviewQuestionByKey(key); known && !have[key] && !slices.Contains(list.DismissedKeys, key) {
			list.DismissedKeys = append(list.DismissedKeys, key)
			changed = true
		}
	}
	if changed {
		writeQuestionList(tx, list)
	}
}
