package backend

import (
	"errors"
	"family/cfg"
	"family/vision"
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Bump autoTagVersion when the catalog, the background prompts, or the
// thresholds change; every photo's suggestions are then recomputed.
const (
	autoTagVersion     = 2
	suggestMinProb     = 0.5
	suggestMinCosine   = 0.24
	suggestLogitScale  = 100.0
	maxAutoPhraseChars = 120
)

type catalogTag struct {
	Label  string
	Color  string
	Phrase string
}

var tagCatalog = []catalogTag{
	{"Birthday", "#ec4899", "a birthday party with a cake and candles"},
	{"Beach", "#0ea5e9", "a photo at the beach with sand and ocean"},
	{"Snow", "#94a3b8", "people playing in the snow"},
	{"Swimming", "#06b6d4", "a child swimming in a pool"},
	{"Bath time", "#38bdf8", "a baby in the bath"},
	{"Playground", "#84cc16", "kids on a playground"},
	{"School", "#3b82f6", "a child with a backpack on the first day of school"},
	{"Christmas", "#16a34a", "a christmas tree with presents"},
	{"Halloween", "#f97316", "children in halloween costumes"},
	{"Camping", "#65a30d", "camping with a tent"},
	{"Sports", "#22c55e", "kids playing a team sport"},
	{"Dance", "#a855f7", "a dance or ballet performance"},
	{"Pets", "#eab308", "a pet dog or cat"},
	{"Newborn", "#f472b6", "a newborn baby"},
	{"Graduation", "#6366f1", "a graduation ceremony with cap and gown"},
	{"Holiday meal", "#b45309", "a family holiday dinner at the table"},
}

// Neutral descriptions that soak up probability for ordinary photos, so a
// label only wins when the photo is clearly about it.
var backgroundPrompts = []string{
	"a photo of a person", "a portrait photo", "a photo of people indoors",
	"a photo of people outdoors", "a photo of a child", "a photo of a family",
	"a photo of a room", "a photo of a street", "a photo of food",
	"a photo of an animal", "a photo of a vehicle", "a screenshot or document",
}

const (
	SuggestionPending  = 0
	SuggestionAccepted = 1
	SuggestionRejected = 2
)

type TagSuggestion struct {
	Id        int       `json:"id"`
	PhotoId   int       `json:"photoId"`
	FamilyId  int       `json:"familyId"`
	TagId     int       `json:"tagId"`
	Label     string    `json:"label"`
	Score     float64   `json:"score"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func PackTagSuggestion(self *TagSuggestion, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.TagId, buf)
	vpack.String(&self.Label, buf)
	vpack.Float64(&self.Score, buf)
	vpack.Int(&self.Status, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var TagSuggestionBkt = vbolt.Bucket(&cfg.Info, "tag_suggestions", vpack.FInt, PackTagSuggestion)
var SuggestionByPhotoIndex = vbolt.Index(&cfg.Info, "suggestion_by_photo", vpack.FInt, vpack.FInt)
var PendingSuggestionByFamilyIndex = vbolt.Index(&cfg.Info, "pending_suggestion_by_family", vpack.FInt, vpack.FInt)

// SuggestStateBkt records, per photo, which catalog version and family
// generation its suggestions were computed against.
var SuggestStateBkt = vbolt.Bucket(&cfg.Info, "suggest_state", vpack.FInt, vpack.String)

// FamilySuggestGenBkt is bumped when a family changes a tag's phrase, which
// changes the prompt pool for all of its photos.
var FamilySuggestGenBkt = vbolt.Bucket(&cfg.Info, "family_suggest_gen", vpack.FInt, vpack.Int)

func suggestKey(tx *vbolt.Tx, familyId int) string {
	var gen int
	vbolt.Read(tx, FamilySuggestGenBkt, familyId, &gen)
	return fmt.Sprintf("%d:%d", autoTagVersion, gen)
}

func suggestionsOutdated(tx *vbolt.Tx, photoId, familyId int) bool {
	var key string
	vbolt.Read(tx, SuggestStateBkt, photoId, &key)
	return key != suggestKey(tx, familyId)
}

func writeSuggestionTx(tx *vbolt.Tx, s *TagSuggestion) {
	vbolt.Write(tx, TagSuggestionBkt, s.Id, s)
	vbolt.SetTargetSingleTerm(tx, SuggestionByPhotoIndex, s.Id, s.PhotoId)
	if s.Status == SuggestionPending {
		vbolt.SetTargetSingleTerm(tx, PendingSuggestionByFamilyIndex, s.Id, s.FamilyId)
	} else {
		vbolt.DeleteTargetTerms(tx, PendingSuggestionByFamilyIndex, s.Id)
	}
}

func deleteSuggestionTx(tx *vbolt.Tx, id int) {
	vbolt.Delete(tx, TagSuggestionBkt, id)
	vbolt.DeleteTargetTerms(tx, SuggestionByPhotoIndex, id)
	vbolt.DeleteTargetTerms(tx, PendingSuggestionByFamilyIndex, id)
}

func photoSuggestions(tx *vbolt.Tx, photoId int) (out []TagSuggestion) {
	var ids []int
	vbolt.ReadTermTargets(tx, SuggestionByPhotoIndex, photoId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, TagSuggestionBkt, ids, &out)
	return
}

func pendingFamilySuggestions(tx *vbolt.Tx, familyId int) (out []TagSuggestion) {
	var ids []int
	vbolt.ReadTermTargets(tx, PendingSuggestionByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, TagSuggestionBkt, ids, &out)
	return
}

func deletePhotoSuggestionsTx(tx *vbolt.Tx, photoId int) {
	for _, s := range photoSuggestions(tx, photoId) {
		deleteSuggestionTx(tx, s.Id)
	}
	vbolt.Delete(tx, SuggestStateBkt, photoId)
}

func deleteTagSuggestionsTx(tx *vbolt.Tx, tag Tag) {
	for _, s := range pendingFamilySuggestions(tx, tag.FamilyId) {
		if s.TagId == tag.Id {
			deleteSuggestionTx(tx, s.Id)
		}
	}
}

// suggestionTarget is one label a photo can be scored against: a family tag
// with a phrase, or a catalog entry the family has no tag for yet.
type suggestionTarget struct {
	tagId  int
	label  string
	phrase string
}

func (t suggestionTarget) key() string {
	if t.tagId > 0 {
		return fmt.Sprintf("tag:%d", t.tagId)
	}
	return "cat:" + strings.ToLower(t.label)
}

func suggestionKeyOf(s TagSuggestion) string {
	return suggestionTarget{tagId: s.TagId, label: s.Label}.key()
}

func suggestionTargets(tx *vbolt.Tx, familyId int) []suggestionTarget {
	var targets []suggestionTarget
	byName := map[string]Tag{}
	for _, tag := range getTagsByFamily(tx, familyId) {
		byName[strings.ToLower(tag.Name)] = tag
		if tag.AutoPhrase != "" {
			targets = append(targets, suggestionTarget{tag.Id, tag.Name, tag.AutoPhrase})
		}
	}
	for _, c := range tagCatalog {
		if tag, ok := byName[strings.ToLower(c.Label)]; ok {
			if tag.AutoPhrase == "" {
				targets = append(targets, suggestionTarget{tag.Id, tag.Name, c.Phrase})
			}
			continue
		}
		targets = append(targets, suggestionTarget{0, c.Label, c.Phrase})
	}
	return targets
}

var phraseVectors = struct {
	sync.Mutex
	byText map[string][]float32
}{byText: map[string][]float32{}}

func embedPhrases(client *visionClient, phrases []string) ([][]float32, error) {
	out := make([][]float32, len(phrases))
	var missing []string
	phraseVectors.Lock()
	for i, p := range phrases {
		if v, ok := phraseVectors.byText[p]; ok {
			out[i] = v
		} else {
			missing = append(missing, p)
		}
	}
	phraseVectors.Unlock()
	if len(missing) > 0 {
		vectors, err := client.embedTexts("clip", missing)
		if err != nil {
			return nil, err
		}
		if len(vectors) != len(missing) {
			return nil, fmt.Errorf("vision daemon returned %d vectors for %d phrases", len(vectors), len(missing))
		}
		phraseVectors.Lock()
		if len(phraseVectors.byText) > 4096 {
			phraseVectors.byText = map[string][]float32{}
		}
		for i, p := range missing {
			phraseVectors.byText[p] = vectors[i]
		}
		for i, p := range phrases {
			out[i] = phraseVectors.byText[p]
		}
		phraseVectors.Unlock()
	}
	return out, nil
}

type labelScore struct {
	prob   float64
	cosine float64
}

// scoreLabels is a zero-shot classification: a softmax over the targets and
// the background prompts, scaled the way CLIP was trained.
func scoreLabels(image []float32, targets, background [][]float32) []labelScore {
	all := append(append([][]float32{}, targets...), background...)
	logits := make([]float64, len(all))
	top := math.Inf(-1)
	for i, v := range all {
		logits[i] = suggestLogitScale * float64(vision.Dot(image, v))
		top = max(top, logits[i])
	}
	var sum float64
	for i := range logits {
		logits[i] = math.Exp(logits[i] - top)
		sum += logits[i]
	}
	scores := make([]labelScore, len(targets))
	for i := range targets {
		scores[i] = labelScore{prob: logits[i] / sum, cosine: float64(vision.Dot(image, targets[i]))}
	}
	return scores
}

func suggestTagsForPhoto(db *vbolt.DB, client *visionClient, photoId int) error {
	var img Image
	var embedding PhotoEmbedding
	var found, outdated bool
	var targets []suggestionTarget
	var generation string
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		img = GetImageById(tx, photoId)
		embedding, found = GetPhotoEmbedding(tx, photoId)
		if img.Id != 0 {
			outdated = suggestionsOutdated(tx, img.Id, img.FamilyId)
			generation = suggestKey(tx, img.FamilyId)
			targets = suggestionTargets(tx, img.FamilyId)
		}
	})
	if img.Id == 0 || img.Status != 0 || !found || embeddingOutdated(embedding, true) || !outdated {
		return nil
	}

	phrases := make([]string, len(targets))
	for i, t := range targets {
		phrases[i] = t.phrase
	}
	vectors, err := embedPhrases(client, append(phrases, backgroundPrompts...))
	if errors.Is(err, errVisionUnavailable) {
		noteVisionReachable(false)
		return errRetryLater
	}
	if errors.Is(err, errRetryLater) {
		return errRetryLater
	}
	if err != nil {
		log.Printf("[VISION] Could not embed tag phrases: %v", err)
		return nil
	}
	scores := scoreLabels(embedding.Vector, vectors[:len(targets)], vectors[len(targets):])
	stale := false

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, photoId)
		if img.Id == 0 || img.Status != 0 {
			return
		}
		if suggestKey(tx, img.FamilyId) != generation {
			stale = true
			return
		}
		existing := map[string]bool{}
		for _, s := range photoSuggestions(tx, img.Id) {
			// Only reviewed decisions survive a new scoring pass. Pending results
			// are replaced, including their scores, against the current prompt pool.
			if s.Status == SuggestionPending {
				deleteSuggestionTx(tx, s.Id)
				continue
			}
			existing[suggestionKeyOf(s)] = true
			existing["label:"+strings.ToLower(s.Label)] = true
		}
		hasTag := map[int]bool{}
		for _, id := range GetPhotoTagIds(tx, img.Id) {
			hasTag[id] = true
		}
		for i, t := range targets {
			if scores[i].prob <= suggestMinProb || scores[i].cosine < suggestMinCosine {
				continue
			}
			if existing[t.key()] || existing["label:"+strings.ToLower(t.label)] || (t.tagId > 0 && hasTag[t.tagId]) {
				continue
			}
			writeSuggestionTx(tx, &TagSuggestion{
				Id:      vbolt.NextIntId(tx, TagSuggestionBkt),
				PhotoId: img.Id, FamilyId: img.FamilyId,
				TagId: t.tagId, Label: t.label, Score: scores[i].prob,
				Status: SuggestionPending, CreatedAt: time.Now(),
			})
		}
		vbolt.Write(tx, SuggestStateBkt, img.Id, &generation)
		vbolt.TxCommit(tx)
	})
	if stale {
		return errRetryLater
	}
	return nil
}

func photosNeedingSuggestions(tx *vbolt.Tx) (ids []int) {
	vbolt.IterateAll(tx, PhotoEmbeddingBkt, func(photoId int, e PhotoEmbedding) bool {
		if suggestionsOutdated(tx, photoId, e.FamilyId) {
			ids = append(ids, photoId)
		}
		return true
	})
	return
}

// bumpFamilySuggestionsTx marks every photo in the family for rescoring. The
// caller queues them once its transaction commits.
func bumpFamilySuggestionsTx(tx *vbolt.Tx, familyId int) {
	var gen int
	vbolt.Read(tx, FamilySuggestGenBkt, familyId, &gen)
	gen++
	vbolt.Write(tx, FamilySuggestGenBkt, familyId, &gen)
}

func queueFamilySuggestions(db *vbolt.DB, familyId int) {
	if globalVisionWorker == nil {
		return
	}
	var ids []int
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.ReadTermTargets(tx, EmbeddingByFamilyIndex, familyId, &ids, vbolt.Window{})
	})
	globalVisionWorker.add(ids...)
}

type SuggestionGroup struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	TagId       int             `json:"tagId"`
	Color       string          `json:"color"`
	FamilyId    int             `json:"familyId"`
	Suggestions []TagSuggestion `json:"suggestions"`
}

type GetTagSuggestionsResponse struct {
	Enabled bool              `json:"enabled"`
	Groups  []SuggestionGroup `json:"groups"`
	Total   int               `json:"total"`
}

func isCatalogLabel(name string) bool {
	for _, c := range tagCatalog {
		if strings.EqualFold(c.Label, strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func catalogColor(label string) string {
	for _, c := range tagCatalog {
		if strings.EqualFold(c.Label, label) {
			return c.Color
		}
	}
	return "#6366f1"
}

func GetTagSuggestions(ctx *vbeam.Context, req Empty) (resp GetTagSuggestionsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	resp.Enabled = globalVisionClient != nil
	resp.Groups = []SuggestionGroup{}
	for _, familyId := range familiesVisibleTo(ctx.Tx, user) {
		if !CanAccessFamily(ctx.Tx, user, familyId, AccessContribute) {
			continue
		}
		byKey := map[string]*SuggestionGroup{}
		var order []string
		for _, s := range pendingFamilySuggestions(ctx.Tx, familyId) {
			if img := GetImageById(ctx.Tx, s.PhotoId); img.Id == 0 || img.Status != 0 {
				continue
			}
			key := fmt.Sprintf("%d/%s", familyId, suggestionKeyOf(s))
			g := byKey[key]
			if g == nil {
				g = &SuggestionGroup{Key: key, Label: s.Label, TagId: s.TagId, FamilyId: familyId, Color: catalogColor(s.Label)}
				if s.TagId > 0 {
					tag := getTagById(ctx.Tx, s.TagId)
					g.Label, g.Color = tag.Name, tag.Color
				}
				byKey[key] = g
				order = append(order, key)
			}
			g.Suggestions = append(g.Suggestions, s)
			resp.Total++
		}
		for _, key := range order {
			g := byKey[key]
			sort.Slice(g.Suggestions, func(i, j int) bool { return g.Suggestions[i].Score > g.Suggestions[j].Score })
			resp.Groups = append(resp.Groups, *g)
		}
	}
	sort.SliceStable(resp.Groups, func(i, j int) bool {
		return len(resp.Groups[i].Suggestions) > len(resp.Groups[j].Suggestions)
	})
	return
}

type SuggestedTag struct {
	Id    int    `json:"id"`
	Label string `json:"label"`
	Color string `json:"color"`
}

func pendingPhotoSuggestions(tx *vbolt.Tx, photoId int) []SuggestedTag {
	out := []SuggestedTag{}
	for _, s := range photoSuggestions(tx, photoId) {
		if s.Status != SuggestionPending {
			continue
		}
		chip := SuggestedTag{Id: s.Id, Label: s.Label, Color: catalogColor(s.Label)}
		if s.TagId > 0 {
			tag := getTagById(tx, s.TagId)
			chip.Label, chip.Color = tag.Name, tag.Color
		}
		out = append(out, chip)
	}
	return out
}

type SuggestionIdsRequest struct {
	Ids []int `json:"ids"`
}

type SuggestionIdsResponse struct {
	Updated int `json:"updated"`
}

func loadOwnSuggestions(tx *vbolt.Tx, user User, ids []int) ([]TagSuggestion, error) {
	var out []TagSuggestion
	for _, id := range ids {
		var s TagSuggestion
		if !vbolt.Read(tx, TagSuggestionBkt, id, &s) || !CanAccessFamily(tx, user, s.FamilyId, AccessContribute) {
			return nil, errors.New("Suggestion not found")
		}
		out = append(out, s)
	}
	return out, nil
}

// tagForSuggestionTx returns the family tag a suggestion refers to, creating
// it from the catalog (or reusing a same-named tag) the first time.
func tagForSuggestionTx(tx *vbolt.Tx, s TagSuggestion) Tag {
	if s.TagId > 0 {
		return getTagById(tx, s.TagId)
	}
	for _, tag := range getTagsByFamily(tx, s.FamilyId) {
		if strings.EqualFold(tag.Name, s.Label) {
			return tag
		}
	}
	tag := Tag{
		Id: vbolt.NextIntId(tx, TagBkt), FamilyId: s.FamilyId,
		Name: s.Label, Color: catalogColor(s.Label), CreatedAt: time.Now(),
	}
	vbolt.Write(tx, TagBkt, tag.Id, &tag)
	vbolt.SetTargetSingleTerm(tx, TagByFamilyIndex, tag.Id, tag.FamilyId)
	return tag
}

func AcceptTagSuggestions(ctx *vbeam.Context, req SuggestionIdsRequest) (resp SuggestionIdsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	suggestions, err := loadOwnSuggestions(ctx.Tx, user, req.Ids)
	if err != nil {
		return
	}
	for _, s := range suggestions {
		if s.Status != SuggestionPending {
			continue
		}
		tag := tagForSuggestionTx(ctx.Tx, s)
		if tag.Id == 0 {
			continue
		}
		if !slices.Contains(GetPhotoTagIds(ctx.Tx, s.PhotoId), tag.Id) {
			addTagToPhoto(ctx.Tx, s.PhotoId, tag.Id, s.FamilyId)
		}
		s.TagId, s.Status = tag.Id, SuggestionAccepted
		writeSuggestionTx(ctx.Tx, &s)
		resp.Updated++
	}
	vbolt.TxCommit(ctx.Tx)
	return
}

func RejectTagSuggestions(ctx *vbeam.Context, req SuggestionIdsRequest) (resp SuggestionIdsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	suggestions, err := loadOwnSuggestions(ctx.Tx, user, req.Ids)
	if err != nil {
		return
	}
	for _, s := range suggestions {
		if s.Status != SuggestionPending {
			continue
		}
		s.Status = SuggestionRejected
		writeSuggestionTx(ctx.Tx, &s)
		resp.Updated++
	}
	vbolt.TxCommit(ctx.Tx)
	return
}

func RegisterTagSuggestionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetTagSuggestions)
	vbeam.RegisterProc(app, AcceptTagSuggestions)
	vbeam.RegisterProc(app, RejectTagSuggestions)
}
