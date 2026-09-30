package backend

import (
	"context"
	"errors"
	"family/cfg"
	"family/vision"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const (
	sentenceModelId          = "all-minilm-l6-v2"
	milestoneEmbedVersion    = 1
	siblingMatchMinScore     = 0.45
	categoryNeighborMinScore = 0.6
	milestoneSearchMinScore  = 0.35
	milestonePhotoWindow     = 14 * 24 * time.Hour
	milestonePhotoLimit      = 8
)

type MilestoneEmbedding struct {
	MilestoneId int
	PersonId    int
	FamilyId    int
	Model       string
	Version     int
	Text        string
	Vector      []float32
}

func PackMilestoneEmbedding(self *MilestoneEmbedding, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.MilestoneId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Model, buf)
	vpack.Int(&self.Version, buf)
	vpack.String(&self.Text, buf)
	packFloat32Slice(&self.Vector, buf)
}

var MilestoneEmbeddingBkt = vbolt.Bucket(&cfg.Info, "milestone_embeddings", vpack.FInt, PackMilestoneEmbedding)

func GetMilestoneEmbedding(tx *vbolt.Tx, milestoneId int) (e MilestoneEmbedding, ok bool) {
	ok = vbolt.Read(tx, MilestoneEmbeddingBkt, milestoneId, &e)
	return
}

func milestoneEmbeddingOutdated(m Milestone, e MilestoneEmbedding, found bool) bool {
	return !found || e.Model != sentenceModelId || e.Version < milestoneEmbedVersion ||
		e.Text != m.Description || e.PersonId != m.PersonId || len(e.Vector) == 0
}

func deleteMilestoneEmbeddingTx(tx *vbolt.Tx, milestoneId int) {
	vbolt.Delete(tx, MilestoneEmbeddingBkt, milestoneId)
}

func embedSentences(client *visionClient, texts []string) ([][]float32, error) {
	if client == nil {
		return nil, errVisionUnavailable
	}
	vectors, err := client.embedTexts("sentence", texts)
	if errors.Is(err, errVisionUnavailable) {
		noteVisionReachable(false)
	} else if err == nil {
		noteVisionReachable(true)
		if len(vectors) != len(texts) {
			return nil, fmt.Errorf("vision daemon returned %d vectors for %d texts", len(vectors), len(texts))
		}
	}
	return vectors, err
}

func embedMilestone(db *vbolt.DB, client *visionClient, milestoneId int) error {
	var m Milestone
	var current MilestoneEmbedding
	var found bool
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		m = GetMilestoneById(tx, milestoneId)
		current, found = GetMilestoneEmbedding(tx, milestoneId)
	})
	if m.Id == 0 || !milestoneEmbeddingOutdated(m, current, found) {
		return nil
	}
	vectors, err := embedSentences(client, []string{m.Description})
	if errors.Is(err, errVisionUnavailable) || errors.Is(err, errRetryLater) {
		return errRetryLater
	}
	if err != nil {
		log.Printf("[VISION] Milestone %d embedding failed: %v", milestoneId, err)
		return nil
	}
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		if GetMilestoneById(tx, milestoneId).Id == 0 {
			return
		}
		e := MilestoneEmbedding{
			MilestoneId: m.Id, PersonId: m.PersonId, FamilyId: m.FamilyId,
			Model: sentenceModelId, Version: milestoneEmbedVersion, Text: m.Description, Vector: vectors[0],
		}
		vbolt.Write(tx, MilestoneEmbeddingBkt, m.Id, &e)
		vbolt.TxCommit(tx)
	})
	return nil
}

func milestonesNeedingEmbedding(db *vbolt.DB) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, MilestoneBkt, func(_ int, m Milestone) bool {
			e, found := GetMilestoneEmbedding(tx, m.Id)
			if milestoneEmbeddingOutdated(m, e, found) {
				ids = append(ids, m.Id)
			}
			return true
		})
	})
	if len(ids) > 0 {
		LogInfo(LogCategoryWorker, "Queued milestones for text embedding", map[string]interface{}{"count": len(ids)})
	}
	return
}

var globalMilestoneWorker *backlogWorker[int]

func startMilestoneWorker(db *vbolt.DB, client *visionClient) {
	if globalMilestoneWorker != nil {
		return
	}
	globalMilestoneWorker = newBacklogWorker("Milestone worker", func(id int) error {
		return embedMilestone(db, client, id)
	})
	globalMilestoneWorker.run(func() []int { return milestonesNeedingEmbedding(db) })
}

func QueueMilestoneEmbedding(milestoneId int) {
	if globalMilestoneWorker != nil {
		globalMilestoneWorker.add(milestoneId)
	}
}

func MilestoneQueueLength() int {
	if globalMilestoneWorker == nil {
		return 0
	}
	return globalMilestoneWorker.length()
}

func StopMilestoneWorker(ctx context.Context) bool {
	if globalMilestoneWorker == nil {
		return true
	}
	return globalMilestoneWorker.stopWait(ctx)
}

// Example phrasings per category. A new milestone takes the category of the
// closest example, unless the family has already filed a near-identical one.
var categoryExamples = map[string][]string{
	"development": {"rolled over", "started crawling", "took first steps", "started talking in sentences", "learned to count", "grew taller", "learned a new skill"},
	"behavior":    {"threw a tantrum", "started sleeping through the night", "shares toys nicely", "is shy with strangers", "new funny habit", "refuses to eat vegetables"},
	"health":      {"had a fever", "doctor checkup and vaccines", "broke a bone", "got stitches", "lost a tooth", "got braces", "allergic reaction"},
	"achievement": {"won an award", "learned to ride a bike", "finished a big project", "made the team", "passed a test", "potty trained"},
	"first":       {"first day of school", "first haircut", "first time at the beach", "first plane trip", "first sleepover", "first job"},
}

var categoryVectors = struct {
	sync.Mutex
	byCategory map[string][][]float32
}{}

func categoryExampleVectors(client *visionClient) (map[string][][]float32, error) {
	categoryVectors.Lock()
	defer categoryVectors.Unlock()
	if categoryVectors.byCategory != nil {
		return categoryVectors.byCategory, nil
	}
	var texts []string
	var owners []string
	for category, examples := range categoryExamples {
		for _, e := range examples {
			texts = append(texts, e)
			owners = append(owners, category)
		}
	}
	vectors, err := embedSentences(client, texts)
	if err != nil {
		return nil, err
	}
	out := map[string][][]float32{}
	for i, v := range vectors {
		out[owners[i]] = append(out[owners[i]], v)
	}
	categoryVectors.byCategory = out
	return out, nil
}

type SuggestMilestoneCategoryRequest struct {
	Description string `json:"description"`
	PersonId    int    `json:"personId,omitempty"`
}

type SuggestMilestoneCategoryResponse struct {
	// Category is empty when there is no suggestion.
	Category string `json:"category"`
}

func SuggestMilestoneCategory(ctx *vbeam.Context, req SuggestMilestoneCategoryRequest) (resp SuggestMilestoneCategoryResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	text := strings.TrimSpace(req.Description)
	if len(text) < 3 || len(text) > 500 {
		return
	}
	vectors, embedErr := embedSentences(globalVisionClient, []string{text})
	if embedErr != nil {
		return
	}
	query := vectors[0]

	familyId := user.FamilyId
	if req.PersonId > 0 {
		if person := GetPersonById(ctx.Tx, req.PersonId); CanAccessPerson(ctx.Tx, user, person, ScopeMilestones, AccessView) {
			familyId = person.FamilyId
		}
	}
	if CanAccessFamily(ctx.Tx, user, familyId, AccessView) {
		best, bestScore := "", categoryNeighborMinScore
		for _, m := range getFamilyMilestones(ctx.Tx, familyId) {
			if e, ok := GetMilestoneEmbedding(ctx.Tx, m.Id); ok && !milestoneEmbeddingOutdated(m, e, true) {
				if s := float64(vision.Dot(query, e.Vector)); s >= bestScore {
					best, bestScore = m.Category, s
				}
			}
		}
		if best != "" {
			resp.Category = best
			return
		}
	}

	examples, embedErr := categoryExampleVectors(globalVisionClient)
	if embedErr != nil {
		return
	}
	bestScore := math.Inf(-1)
	for category, vecs := range examples {
		for _, v := range vecs {
			if s := float64(vision.Dot(query, v)); s > bestScore {
				resp.Category, bestScore = category, s
			}
		}
	}
	return
}

type SuggestMilestonePhotosRequest struct {
	PersonId      int     `json:"personId"`
	Description   string  `json:"description"`
	InputType     string  `json:"inputType"`
	MilestoneDate *string `json:"milestoneDate,omitempty"`
	AgeYears      *int    `json:"ageYears,omitempty"`
	AgeMonths     *int    `json:"ageMonths,omitempty"`
	// Photos already attached, left out of the suggestions.
	ExcludeIds []int `json:"excludeIds,omitempty"`
}

type SuggestMilestonePhotosResponse struct {
	PhotoIds []int `json:"photoIds"`
	// Ranked is false when the photos are only the ones nearest the date,
	// because the description could not be compared with them.
	Ranked bool `json:"ranked"`
}

// SuggestMilestonePhotos finds photos of the person from about two weeks
// either side of the milestone, best match to the description first.
func SuggestMilestonePhotos(ctx *vbeam.Context, req SuggestMilestonePhotosRequest) (resp SuggestMilestonePhotosResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	resp.PhotoIds = []int{}
	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopePhotos, AccessView) {
		err = errors.New("Person not found or not in your family")
		return
	}
	date, dateErr := resolveEntryDate("Milestone", req.InputType, req.MilestoneDate, req.AgeYears, req.AgeMonths, person.Birthday)
	if dateErr != nil {
		return
	}

	excluded := map[int]bool{}
	for _, id := range req.ExcludeIds {
		excluded[id] = true
	}
	var candidates []Image
	stream := newPhotoStream(ImageByPersonDateIndex, person.Id, photoKey{seconds: date.Add(milestonePhotoWindow).Unix(), id: math.MaxInt})
	floor := date.Add(-milestonePhotoWindow).Unix()
	for stream.peek(ctx.Tx) {
		img := stream.pop()
		if img.PhotoDate.Unix() < floor {
			break
		}
		if img.Status == 0 && !excluded[img.Id] && CanAccessPhoto(ctx.Tx, user, img, AccessView) {
			candidates = append(candidates, img)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return absDuration(candidates[i].PhotoDate.Sub(date)) < absDuration(candidates[j].PhotoDate.Sub(date))
	})

	if text := strings.TrimSpace(req.Description); text != "" && len(candidates) > 1 {
		if query, embedErr := embedQuery(text); embedErr == nil {
			scores := map[int]float32{}
			for _, img := range candidates {
				if v, ok := vectorsForFamily(ctx.Tx, img.FamilyId)[img.Id]; ok {
					scores[img.Id] = vision.Dot(query, v)
				}
			}
			sort.SliceStable(candidates, func(i, j int) bool { return scores[candidates[i].Id] > scores[candidates[j].Id] })
			resp.Ranked = true
		}
	}
	for _, img := range candidates[:min(len(candidates), milestonePhotoLimit)] {
		resp.PhotoIds = append(resp.PhotoIds, img.Id)
	}
	return
}

type GetMilestoneMatchesRequest struct {
	MilestoneId int `json:"milestoneId"`
}

type MilestoneMatch struct {
	Person    Person    `json:"person"`
	Milestone Milestone `json:"milestone"`
	AgeMonths int       `json:"ageMonths"`
}

type GetMilestoneMatchesResponse struct {
	AgeMonths int              `json:"ageMonths"`
	Matches   []MilestoneMatch `json:"matches"`
}

func ageInMonths(birthday, at time.Time) int {
	if birthday.IsZero() {
		return -1
	}
	months := (at.Year()-birthday.Year())*12 + int(at.Month()-birthday.Month())
	if at.Day() < birthday.Day() {
		months--
	}
	return months
}

type embeddedMilestone struct {
	milestone Milestone
	vector    []float32
}

func embeddedMilestonesOf(tx *vbolt.Tx, personId int) (out []embeddedMilestone) {
	for _, m := range GetPersonMilestonesTx(tx, personId) {
		if e, ok := GetMilestoneEmbedding(tx, m.Id); ok && !milestoneEmbeddingOutdated(m, e, true) {
			out = append(out, embeddedMilestone{m, e.Vector})
		}
	}
	return
}

func bestMatch(v []float32, pool []embeddedMilestone) (int, float64) {
	best, bestScore := -1, math.Inf(-1)
	for i, p := range pool {
		if s := float64(vision.Dot(v, p.vector)); s > bestScore {
			best, bestScore = i, s
		}
	}
	return best, bestScore
}

// GetMilestoneMatches pairs a milestone with the same milestone in the lives
// of the person's siblings and cousins on the family roster, however each was
// worded. A pair counts only when each is the other's closest milestone.
func GetMilestoneMatches(ctx *vbeam.Context, req GetMilestoneMatchesRequest) (resp GetMilestoneMatchesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	milestone, err := GetMilestoneForUser(ctx.Tx, req.MilestoneId, user, AccessView)
	if err != nil {
		return
	}
	resp.Matches = []MilestoneMatch{}
	person := GetPersonById(ctx.Tx, milestone.PersonId)
	resp.AgeMonths = ageInMonths(person.Birthday, milestone.MilestoneDate)
	own := embeddedMilestonesOf(ctx.Tx, person.Id)
	var target []float32
	for _, m := range own {
		if m.milestone.Id == milestone.Id {
			target = m.vector
		}
	}
	if target == nil {
		return
	}

	for _, other := range GetVisiblePeople(ctx.Tx, user) {
		if other.Id == person.Id || other.IsPregnancy || other.Birthday.IsZero() {
			continue
		}
		if !CanAccessPerson(ctx.Tx, user, other, ScopeMilestones, AccessView) {
			continue
		}
		theirs := embeddedMilestonesOf(ctx.Tx, other.Id)
		i, score := bestMatch(target, theirs)
		if i < 0 || score < siblingMatchMinScore {
			continue
		}
		if back, _ := bestMatch(theirs[i].vector, own); back < 0 || own[back].milestone.Id != milestone.Id {
			continue
		}
		match := theirs[i].milestone
		other.Age = calculateAge(other.Birthday)
		resp.Matches = append(resp.Matches, MilestoneMatch{
			Person: other, Milestone: match, AgeMonths: ageInMonths(other.Birthday, match.MilestoneDate),
		})
	}
	sort.Slice(resp.Matches, func(i, j int) bool { return resp.Matches[i].Person.Birthday.Before(resp.Matches[j].Person.Birthday) })
	return
}

func visibleMilestones(tx *vbolt.Tx, user User) (out []Milestone) {
	seen := map[int]bool{}
	add := func(ms []Milestone) {
		for _, m := range ms {
			if !seen[m.Id] {
				seen[m.Id] = true
				out = append(out, m)
			}
		}
	}
	for _, familyId := range familiesVisibleTo(tx, user) {
		add(getFamilyMilestones(tx, familyId))
	}
	for _, person := range linkedPeopleVisibleTo(tx, user, ScopeMilestones) {
		add(GetPersonMilestonesTx(tx, person.Id))
	}
	return
}

// semanticMilestoneScores ranks the candidates by meaning against the
// query, or returns nil when the daemon can't embed it.
func semanticMilestoneScores(tx *vbolt.Tx, query string, candidates []Milestone) map[int]float64 {
	vectors, err := embedSentences(globalVisionClient, []string{query})
	if err != nil {
		return nil
	}
	scores := map[int]float64{}
	for _, m := range candidates {
		if e, ok := GetMilestoneEmbedding(tx, m.Id); ok && !milestoneEmbeddingOutdated(m, e, true) {
			scores[m.Id] = float64(vision.Dot(vectors[0], e.Vector))
		}
	}
	return scores
}

func RegisterMilestoneAnalysisMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, SuggestMilestoneCategory)
	vbeam.RegisterProc(app, SuggestMilestonePhotos)
	vbeam.RegisterProc(app, GetMilestoneMatches)
}
