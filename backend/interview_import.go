package backend

import (
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"family/vision"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterInterviewImportMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, PreviewInterviewImport)
	vbeam.RegisterProc(app, ImportInterviews)
	vbeam.RegisterProc(app, SaveHistoricalInterview)
}

const (
	maxImportText       = 200_000
	maxImportInterviews = 60
	importSection       = "More questions"
	matchExact          = "exact"
	matchClose          = "close"
	matchGuess          = "guess"
	matchNone           = "none"
	closeMatchMinScore  = 0.5
	guessMatchMinScore  = 0.62
)

// Parsing

type ParsedRow struct {
	Question string
	Answer   string
}

type ParsedInterview struct {
	Header string
	When   InterviewWhen
	Rows   []ParsedRow
}

// InterviewWhen is what a header says about when an interview happened. Age
// is -1 when unknown; Year alone means only the year is known.
type InterviewWhen struct {
	Date time.Time
	Year int
	Age  int
}

func (w InterviewWhen) known() bool { return !w.Date.IsZero() || w.Year > 0 || w.Age >= 0 }

var (
	yearOnly    = regexp.MustCompile(`^(?:19|20)\d\d$`)
	ageHeader   = regexp.MustCompile(`^(?:age\s*)?(\d{1,2})\s*(?:years?(?:\s*old)?|yrs?|yo|y/o|y\.o\.)?$`)
	ageWord     = regexp.MustCompile(`^age\s*(\d{1,2})$`)
	yearInText  = regexp.MustCompile(`\b((?:19|20)\d\d)\b`)
	ageInText   = regexp.MustCompile(`\b(?:age|aged)\s*(\d{1,2})\b|\b(\d{1,2})\s*(?:years? old|yrs? old|yo|y/o)\b`)
	headerNoise = regexp.MustCompile(`(?i)\b(interview|annual|birthday|questions?|answers?)\b`)
)

var headerDateLayouts = []string{
	"2006-01-02", "2006/01/02", "1/2/2006", "01/02/2006", "1/2/06", "1-2-2006",
	"January 2, 2006", "Jan 2, 2006", "January 2 2006", "Jan 2 2006", "2 January 2006",
}

func parseInterviewHeader(raw string) (InterviewWhen, bool) {
	when := InterviewWhen{Age: -1}
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "#*:-–—=()[]"))
	if s == "" || len(s) > 60 {
		return when, false
	}
	lower := strings.ToLower(s)
	for _, layout := range []string{"January 2006", "Jan 2006", "1/2006"} {
		if d, err := time.Parse(layout, s); err == nil {
			when.Year = d.Year()
			return when, true
		}
	}
	if yearOnly.MatchString(lower) {
		when.Year, _ = strconv.Atoi(lower)
		return when, true
	}
	if m := ageHeader.FindStringSubmatch(lower); m != nil && (strings.Contains(lower, "age") || len(lower) > len(m[1])) {
		when.Age, _ = strconv.Atoi(m[1])
		return when, true
	}
	for _, layout := range headerDateLayouts {
		if d, err := time.Parse(layout, s); err == nil {
			when.Date = dayStart(d)
			return when, true
		}
	}
	stripped := strings.TrimSpace(headerNoise.ReplaceAllString(lower, " "))
	if stripped != lower && stripped != "" {
		if inner, ok := parseInterviewHeader(stripped); ok {
			return inner, true
		}
	}
	if !strings.ContainsAny(s, ":?") && len(strings.Fields(s)) <= 6 {
		if m := ageInText.FindStringSubmatch(lower); m != nil {
			n := m[1]
			if n == "" {
				n = m[2]
			}
			when.Age, _ = strconv.Atoi(n)
			if y := yearInText.FindString(lower); y != "" {
				when.Year, _ = strconv.Atoi(y)
			}
			return when, true
		}
		if y := yearInText.FindString(lower); y != "" && strings.Contains(lower, "interview") {
			when.Year, _ = strconv.Atoi(y)
			return when, true
		}
	}
	return when, false
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimRight(line, " \t\r"))
		}
	}
	return lines
}

func parseInterviewText(text string, isCSV bool) []ParsedInterview {
	lines := nonEmptyLines(text)
	if len(lines) == 0 {
		return nil
	}
	if isCSV {
		records, err := readCSV(text)
		if err == nil {
			return parseTable(records)
		}
	}
	tabbed := 0
	for _, line := range lines {
		if strings.Contains(line, "\t") {
			tabbed++
		}
	}
	if len(lines) >= 2 && tabbed*10 >= len(lines)*8 {
		var records [][]string
		for _, line := range lines {
			records = append(records, strings.Split(line, "\t"))
		}
		return parseTable(records)
	}
	return parseLines(lines)
}

func readCSV(text string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	return r.ReadAll()
}

func cell(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

func parseTable(records [][]string) []ParsedInterview {
	if len(records) == 0 {
		return nil
	}
	header := records[0]
	width := 0
	for _, r := range records {
		width = max(width, len(r))
	}

	headerWhens := 0
	for i := 1; i < width; i++ {
		if _, ok := parseInterviewHeader(cell(header, i)); ok {
			headerWhens++
		}
	}
	columnWhens := 0
	for _, r := range records[1:] {
		if _, ok := parseInterviewHeader(cell(r, 0)); ok {
			columnWhens++
		}
	}

	if columnWhens > headerWhens && columnWhens*2 >= len(records)-1 {
		var out []ParsedInterview
		for _, r := range records[1:] {
			when, _ := parseInterviewHeader(cell(r, 0))
			iv := ParsedInterview{Header: cell(r, 0), When: when}
			for i := 1; i < width; i++ {
				if q := cell(header, i); q != "" {
					iv.Rows = append(iv.Rows, ParsedRow{Question: q, Answer: cell(r, i)})
				}
			}
			out = append(out, iv)
		}
		return out
	}

	start := 1
	if headerWhens == 0 {
		start = 0
	}
	var out []ParsedInterview
	for i := 1; i < width; i++ {
		iv := ParsedInterview{When: InterviewWhen{Age: -1}}
		if start == 1 {
			iv.Header = cell(header, i)
			iv.When, _ = parseInterviewHeader(iv.Header)
		}
		for _, r := range records[start:] {
			if q := cell(r, 0); q != "" {
				iv.Rows = append(iv.Rows, ParsedRow{Question: q, Answer: cell(r, i)})
			}
		}
		out = append(out, iv)
	}
	return out
}

func splitQuestionLine(line string) (string, string, bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimLeft(s, "-*•·–— \t")
	s = strings.TrimSpace(strings.TrimLeft(s, "0123456789"))
	s = strings.TrimSpace(strings.TrimLeft(s, ".)"))
	cuts := []struct {
		sep  string
		keep bool
	}{{"?", true}, {":", false}, {" - ", false}, {" – ", false}, {" — ", false}, {" = ", false}}
	best := -1
	bestSep := ""
	bestKeep := false
	for _, c := range cuts {
		if i := strings.Index(s, c.sep); i > 0 && (best < 0 || i < best) {
			best, bestSep, bestKeep = i, c.sep, c.keep
		}
	}
	if best < 0 {
		return s, "", false
	}
	q := strings.TrimSpace(s[:best])
	if bestKeep {
		q += bestSep
	}
	a := strings.TrimSpace(s[best+len(bestSep):])
	return q, a, true
}

func parseLines(lines []string) []ParsedInterview {
	var out []ParsedInterview
	current := ParsedInterview{When: InterviewWhen{Age: -1}}
	flush := func() {
		if len(current.Rows) > 0 || current.When.known() {
			out = append(out, current)
		}
	}

	questionLines := 0
	for i := 0; i+1 < len(lines); i += 2 {
		if strings.HasSuffix(strings.TrimSpace(lines[i]), "?") && !strings.Contains(strings.TrimSpace(lines[i+1]), "?") {
			questionLines++
		}
	}
	alternating := len(lines) >= 2 && questionLines*10 >= (len(lines)/2)*7

	if alternating {
		for i := 0; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if when, ok := parseInterviewHeader(line); ok && !strings.HasSuffix(line, "?") {
				flush()
				current = ParsedInterview{Header: line, When: when}
				continue
			}
			answer := ""
			if i+1 < len(lines) && !strings.HasSuffix(strings.TrimSpace(lines[i+1]), "?") {
				answer = strings.TrimSpace(lines[i+1])
				i++
			}
			current.Rows = append(current.Rows, ParsedRow{Question: line, Answer: answer})
		}
		flush()
		return out
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if when, ok := parseInterviewHeader(trimmed); ok {
			flush()
			current = ParsedInterview{Header: trimmed, When: when}
			continue
		}
		q, a, _ := splitQuestionLine(trimmed)
		if q == "" {
			continue
		}
		if label := strings.ToLower(strings.TrimSpace(q)); (label == "age" || label == "date") && !current.When.known() {
			if when, ok := parseInterviewHeader(label + " " + a); ok || label == "date" {
				if !ok {
					when, ok = parseInterviewHeader(a)
				}
				if ok {
					current.When = when
					continue
				}
			}
		}
		current.Rows = append(current.Rows, ParsedRow{Question: q, Answer: a})
	}
	flush()
	return out
}

// Matching

var questionFiller = map[string]bool{
	"what": true, "whats": true, "what's": true, "who": true, "whos": true, "who's": true, "is": true,
	"are": true, "your": true, "youre": true, "you": true, "the": true, "a": true, "an": true,
	"do": true, "does": true,
	"of": true, "my": true, "q": true,
	"s": true, "thing": true, "things": true,
}

var spellings = map[string]string{"fav": "favorite", "fave": "favorite", "favs": "favorite", "favourite": "favorite", "colour": "color", "colours": "color", "tv": "show", "movies": "movie"}

func questionTokens(text string) []string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "’", "'")
	var tokens []string
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '\'')
	}) {
		word = strings.ReplaceAll(word, "'", "")
		if questionFiller[word] || word == "" {
			continue
		}
		if s, ok := spellings[word]; ok {
			word = s
		}
		if len(word) > 3 && strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") {
			word = strings.TrimSuffix(word, "s")
		}
		tokens = append(tokens, word)
	}
	return tokens
}

func normalizeQuestion(text string) string {
	return strings.Join(questionTokens(text), " ")
}

func tokenOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]bool{}
	for _, t := range a {
		set[t] = true
	}
	shared := 0
	seen := map[string]bool{}
	for _, t := range b {
		if set[t] && !seen[t] {
			shared++
		}
		seen[t] = true
	}
	union := len(set)
	for t := range seen {
		if !set[t] {
			union++
		}
	}
	return float64(shared) / float64(union)
}

type questionMatch struct {
	Ref        string
	Confidence string
}

func questionRef(q InterviewQuestion) string {
	if q.Id > 0 {
		return fmt.Sprintf("id:%d", q.Id)
	}
	return "key:" + q.DefaultKey
}

func matchQuestions(raw []string, candidates []InterviewQuestion, embed func([]string) ([][]float32, error)) []questionMatch {
	matches := make([]questionMatch, len(raw))
	normalized := make([]string, len(candidates))
	tokens := make([][]string, len(candidates))
	for i, q := range candidates {
		tokens[i] = questionTokens(q.Text)
		normalized[i] = strings.Join(tokens[i], " ")
	}

	var unmatched []int
	for i, text := range raw {
		mine := questionTokens(text)
		norm := strings.Join(mine, " ")
		best, bestScore := -1, 0.0
		for j := range candidates {
			if norm != "" && norm == normalized[j] {
				best, bestScore = j, 2
				break
			}
			if s := tokenOverlap(mine, tokens[j]); s > bestScore {
				best, bestScore = j, s
			}
		}
		switch {
		case bestScore >= 2:
			matches[i] = questionMatch{questionRef(candidates[best]), matchExact}
		case bestScore >= closeMatchMinScore:
			matches[i] = questionMatch{questionRef(candidates[best]), matchClose}
		default:
			matches[i] = questionMatch{"", matchNone}
			unmatched = append(unmatched, i)
		}
	}

	if len(unmatched) == 0 || len(candidates) == 0 || embed == nil {
		return matches
	}
	texts := make([]string, 0, len(unmatched)+len(candidates))
	for _, i := range unmatched {
		texts = append(texts, raw[i])
	}
	for _, q := range candidates {
		texts = append(texts, q.Text)
	}
	vectors, err := embed(texts)
	if err != nil {
		return matches
	}
	for k, i := range unmatched {
		best, bestScore := -1, guessMatchMinScore
		for j := range candidates {
			if s := float64(vision.Dot(vectors[k], vectors[len(unmatched)+j])); s >= bestScore {
				best, bestScore = j, s
			}
		}
		if best >= 0 {
			matches[i] = questionMatch{questionRef(candidates[best]), matchGuess}
		}
	}
	return matches
}

// Preview

type PreviewInterviewImportRequest struct {
	PersonId int    `json:"personId"`
	Text     string `json:"text"`
	CSV      bool   `json:"csv"`
}

type ImportRow struct {
	Raw        string `json:"raw"`
	Answer     string `json:"answer"`
	Ref        string `json:"ref"`
	Confidence string `json:"confidence"`
}

type ImportProposal struct {
	Header   string      `json:"header"`
	Date     string      `json:"date"`
	AgeYears int         `json:"ageYears"`
	Rows     []ImportRow `json:"rows"`
}

type PreviewInterviewImportResponse struct {
	Proposals []ImportProposal    `json:"proposals"`
	Questions []InterviewQuestion `json:"questions"`
}

// Only a known day is an exact record date. Preserve partial headers in review.
func proposalDate(when InterviewWhen, birthday time.Time) (string, int) {
	if !when.Date.IsZero() {
		return when.Date.Format("2006-01-02"), -1
	}
	return "", when.Age
}

func previewInterviewImport(tx *vbolt.Tx, person Person, req PreviewInterviewImportRequest, embed func([]string) ([][]float32, error)) (PreviewInterviewImportResponse, error) {
	resp := PreviewInterviewImportResponse{Proposals: []ImportProposal{}}
	if len(req.Text) > maxImportText {
		return resp, errors.New("That's too much text to import at once")
	}
	parsed := parseInterviewText(req.Text, req.CSV)
	if len(parsed) == 0 {
		return resp, errors.New("Nothing to import was found in that text")
	}
	if len(parsed) > maxImportInterviews {
		return resp, fmt.Errorf("That looks like %d interviews; import at most %d at a time", len(parsed), maxImportInterviews)
	}
	resp.Questions = interviewQuestionsResponse(tx, person.FamilyId).Questions

	var raw []string
	index := map[string]int{}
	for _, iv := range parsed {
		for _, r := range iv.Rows {
			key := strings.ToLower(strings.TrimSpace(r.Question))
			if _, ok := index[key]; !ok {
				index[key] = len(raw)
				raw = append(raw, r.Question)
			}
		}
	}
	matches := matchQuestions(raw, resp.Questions, embed)

	for _, iv := range parsed {
		date, age := proposalDate(iv.When, person.Birthday)
		p := ImportProposal{Header: iv.Header, Date: date, AgeYears: age, Rows: []ImportRow{}}
		for _, r := range iv.Rows {
			m := matches[index[strings.ToLower(strings.TrimSpace(r.Question))]]
			p.Rows = append(p.Rows, ImportRow{Raw: r.Question, Answer: r.Answer, Ref: m.Ref, Confidence: m.Confidence})
		}
		resp.Proposals = append(resp.Proposals, p)
	}
	return resp, nil
}

func PreviewInterviewImport(ctx *vbeam.Context, req PreviewInterviewImportRequest) (resp PreviewInterviewImportResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if _, err = ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute); err != nil {
		return
	}
	embed := func(texts []string) ([][]float32, error) { return embedSentences(globalVisionClient, texts) }
	resp, err = previewInterviewImport(ctx.Tx, GetPersonById(ctx.Tx, req.PersonId), req, embed)
	return
}

// Import

type ImportAnswer struct {
	Ref         string `json:"ref"`
	Answer      string `json:"answer"`
	NewText     string `json:"newText"`
	Keep        bool   `json:"keep"`
	RawQuestion string `json:"rawQuestion,omitempty"`
}

type HistoricalInterview struct {
	InputType string         `json:"inputType"`
	Date      string         `json:"date"`
	AgeYears  int            `json:"ageYears"`
	AgeMonths int            `json:"ageMonths"`
	Title     string         `json:"title"`
	Notes     string         `json:"notes"`
	Answers   []ImportAnswer `json:"answers"`
}

type ImportInterviewsRequest struct {
	PersonId   int                   `json:"personId"`
	Interviews []HistoricalInterview `json:"interviews"`
	RequestId  string                `json:"requestId,omitempty"`
}

type ImportInterviewsResponse struct {
	Interviews []Interview `json:"interviews"`
}

type SaveHistoricalInterviewRequest struct {
	PersonId  int                 `json:"personId"`
	Interview HistoricalInterview `json:"interview"`
	RequestId string              `json:"requestId,omitempty"`
}

type questionResolver struct {
	tx       *vbolt.Tx
	familyId int
	byId     map[int]InterviewQuestion
	byKey    map[string]InterviewQuestion
	created  map[string]InterviewQuestion
	position int
}

func newQuestionResolver(tx *vbolt.Tx, familyId int) *questionResolver {
	ensureFamilyQuestions(tx, familyId)
	r := &questionResolver{tx: tx, familyId: familyId, byId: map[int]InterviewQuestion{}, byKey: map[string]InterviewQuestion{}, created: map[string]InterviewQuestion{}}
	for _, q := range familyQuestions(tx, familyId) {
		r.byId[q.Id] = q
		if q.DefaultKey != "" {
			r.byKey[q.DefaultKey] = q
		}
		if !q.OneOff {
			r.position = max(r.position, q.Position+1)
		}
	}
	return r
}

func (r *questionResolver) resolve(a ImportAnswer) (InterviewQuestion, error) {
	switch {
	case strings.HasPrefix(a.Ref, "id:"):
		id, _ := strconv.Atoi(strings.TrimPrefix(a.Ref, "id:"))
		if q, ok := r.byId[id]; ok {
			return q, nil
		}
		return InterviewQuestion{}, ErrInterviewQuestionNotFound
	case strings.HasPrefix(a.Ref, "key:"):
		if q, ok := r.byKey[strings.TrimPrefix(a.Ref, "key:")]; ok {
			return q, nil
		}
		return InterviewQuestion{}, ErrInterviewQuestionNotFound
	}
	text, err := cleanQuestionText(a.NewText)
	if err != nil {
		return InterviewQuestion{}, err
	}
	key := fmt.Sprintf("%v:%s", a.Keep, strings.ToLower(text))
	if q, ok := r.created[key]; ok {
		return q, nil
	}
	q := InterviewQuestion{FamilyId: r.familyId, Text: text, Section: importSection, OneOff: !a.Keep}
	if a.Keep {
		q.Position = r.position
		r.position++
	} else {
		q.Section = oneOffSection
		q.Position = maxFamilyQuestions
	}
	q = createInterviewQuestion(r.tx, q)
	r.created[key] = q
	r.byId[q.Id] = q
	return q, nil
}

func saveHistoricalTx(tx *vbolt.Tx, person Person, r *questionResolver, in HistoricalInterview) (Interview, []int, error) {
	inputType := in.InputType
	if inputType == "" {
		inputType = "date"
	}
	if inputType != "date" {
		return Interview{}, nil, errors.New("Choose a known interview date; an age alone does not identify an exact day")
	}
	date, err := parseInterviewDate(in.Date, person.Birthday)
	if err != nil {
		return Interview{}, nil, err
	}
	title := strings.TrimSpace(in.Title)
	notes := strings.TrimSpace(in.Notes)
	if len([]rune(title)) > maxInterviewTitle || len([]rune(notes)) > maxInterviewNotes {
		return Interview{}, nil, errors.New("The title or notes are too long")
	}

	interview := Interview{
		Id: vbolt.NextIntId(tx, InterviewBkt), FamilyId: person.FamilyId, PersonId: person.Id,
		Date: dayStart(date), Title: title, Notes: notes, Source: InterviewHistorical, Status: InterviewComplete,
		QuestionIds: []int{}, SkippedIds: []int{}, CreatedAt: time.Now(), CompletedAt: time.Now(), Version: 1, Questions: []InterviewQuestionSnapshot{}, OriginKey: newInterviewOrigin(),
	}
	var answerIds []int
	for _, a := range in.Answers {
		text := strings.TrimSpace(a.Answer)
		if text == "" {
			continue
		}
		if len([]rune(text)) > maxInterviewAnswer {
			return Interview{}, nil, errors.New("One of the answers is too long")
		}
		q, err := r.resolve(a)
		if err != nil {
			return Interview{}, nil, err
		}
		for _, id := range interview.QuestionIds {
			if id == q.Id {
				return Interview{}, nil, fmt.Errorf("%q is answered twice in one interview", q.Text)
			}
		}
		interview.QuestionIds = append(interview.QuestionIds, q.Id)
		prompt := renderQuestion(q.Text, person, interview.Date)
		if a.RawQuestion != "" {
			var promptErr error
			prompt, promptErr = cleanQuestionText(a.RawQuestion)
			if promptErr != nil {
				return Interview{}, nil, promptErr
			}
		}
		interview.Questions = append(interview.Questions, InterviewQuestionSnapshot{QuestionId: q.Id, Text: prompt, Section: q.Section})
		m := Milestone{
			Id: vbolt.NextIntId(tx, MilestoneBkt), PersonId: person.Id, FamilyId: person.FamilyId,
			Description: text, Category: CategoryInterview, Context: prompt,
			MilestoneDate: interview.Date, CreatedAt: time.Now(), InterviewId: interview.Id, QuestionId: q.Id,
		}
		writeMilestone(tx, m)
		answerIds = append(answerIds, m.Id)
	}
	if len(interview.QuestionIds) == 0 {
		return Interview{}, nil, errors.New("An interview needs at least one answer")
	}
	writeInterview(tx, interview)
	return interview, answerIds, nil
}

func importInterviewsTx(tx *vbolt.Tx, person Person, interviews []HistoricalInterview) ([]Interview, []int, error) {
	if len(interviews) == 0 {
		return nil, nil, errors.New("Nothing to import")
	}
	if len(interviews) > maxImportInterviews {
		return nil, nil, fmt.Errorf("Import at most %d interviews at a time", maxImportInterviews)
	}
	resolver := newQuestionResolver(tx, person.FamilyId)
	var saved []Interview
	var answerIds []int
	for i, in := range interviews {
		interview, ids, err := saveHistoricalTx(tx, person, resolver, in)
		if err != nil {
			if len(interviews) > 1 {
				err = fmt.Errorf("Interview %d: %w", i+1, err)
			}
			return nil, nil, err
		}
		saved = append(saved, interview)
		answerIds = append(answerIds, ids...)
	}
	if len(resolver.created) > 0 {
		list := familyQuestionList(tx, person.FamilyId)
		list.Revision++
		writeQuestionList(tx, list)
	}
	sort.Slice(saved, func(i, j int) bool { return saved[i].Date.Before(saved[j].Date) })
	return saved, answerIds, nil
}

func ImportInterviews(ctx *vbeam.Context, req ImportInterviewsRequest) (resp ImportInterviewsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if _, err = ActingFamilyForPerson(ctx.Tx, user, req.PersonId, AccessContribute); err != nil {
		return
	}
	vbeam.UseWriteTx(ctx)
	person := GetPersonById(ctx.Tx, req.PersonId)
	receipt, replay, receiptErr := interviewReceipt(ctx.Tx, person.FamilyId, "history", req.RequestId, req)
	if receiptErr != nil {
		err = receiptErr
		return
	}
	if replay {
		for _, id := range receipt.Ids {
			resp.Interviews = append(resp.Interviews, GetInterviewById(ctx.Tx, id))
		}
		return
	}
	var answerIds []int
	resp.Interviews, answerIds, err = importInterviewsTx(ctx.Tx, GetPersonById(ctx.Tx, req.PersonId), req.Interviews)
	if err != nil {
		return
	}
	ids := []int{}
	for _, i := range resp.Interviews {
		ids = append(ids, i.Id)
	}
	saveInterviewReceipt(ctx.Tx, receipt, ids)
	vbolt.TxCommit(ctx.Tx)
	for _, id := range answerIds {
		QueueMilestoneEmbedding(id)
	}
	return
}

func SaveHistoricalInterview(ctx *vbeam.Context, req SaveHistoricalInterviewRequest) (resp InterviewResponse, err error) {
	imported, err := ImportInterviews(ctx, ImportInterviewsRequest{PersonId: req.PersonId, Interviews: []HistoricalInterview{req.Interview}, RequestId: req.RequestId})
	if err != nil {
		return
	}
	resp.Interview = imported.Interviews[0]
	return
}
