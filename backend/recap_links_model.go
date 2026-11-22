package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// A RecapLink is an account-free, read-only copy of a competition recap. It
// holds the snapshot the sender approved, never a pointer to live records, so
// later edits to the event leave it as it was. Revoking deletes it outright.
type RecapLink struct {
	Id        int
	FamilyId  int
	EventId   int
	CreatedBy int
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	Recap     PublishedRecap
}

type PublishedRecap struct {
	EventName     string             `json:"eventName"`
	Subtitle      string             `json:"subtitle"`
	Place         string             `json:"place"`
	SnapshotLabel string             `json:"snapshotLabel"`
	Routines      []PublishedRoutine `json:"routines"`
}

type PublishedRoutine struct {
	Name         string                 `json:"name"`
	Descriptors  string                 `json:"descriptors"`
	Roster       string                 `json:"roster"`
	Summary      string                 `json:"summary"`
	Performances []PublishedPerformance `json:"performances"`
}

type PublishedPerformance struct {
	When    string            `json:"when"`
	Matchup string            `json:"matchup"`
	Results []PublishedResult `json:"results"`
}

type PublishedResult struct {
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Detail string `json:"detail"`
}

func PackPublishedResult(self *PublishedResult, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Kind, buf)
	vpack.String(&self.Text, buf)
	vpack.String(&self.Detail, buf)
}

func PackPublishedPerformance(self *PublishedPerformance, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.When, buf)
	vpack.String(&self.Matchup, buf)
	vpack.Slice(&self.Results, PackPublishedResult, buf)
}

func PackPublishedRoutine(self *PublishedRoutine, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Descriptors, buf)
	vpack.String(&self.Roster, buf)
	vpack.String(&self.Summary, buf)
	vpack.Slice(&self.Performances, PackPublishedPerformance, buf)
}

func PackPublishedRecap(self *PublishedRecap, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.EventName, buf)
	vpack.String(&self.Subtitle, buf)
	vpack.String(&self.Place, buf)
	vpack.String(&self.SnapshotLabel, buf)
	vpack.Slice(&self.Routines, PackPublishedRoutine, buf)
}

func PackRecapLink(self *RecapLink, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.EventId, buf)
	vpack.Int(&self.CreatedBy, buf)
	vpack.String(&self.TokenHash, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Time(&self.ExpiresAt, buf)
	PackPublishedRecap(&self.Recap, buf)
}

var RecapLinkBkt = vbolt.Bucket(&cfg.Info, "recap_links", vpack.FInt, PackRecapLink)

var RecapLinkByHashBkt = vbolt.Bucket(&cfg.Info, "recap_links_by_hash", vpack.StringZ, vpack.Int)

var RecapLinkByEventIndex = vbolt.Index(&cfg.Info, "recap_link_by_event", vpack.FInt, vpack.FInt)

var RecapLinkByFamilyIndex = vbolt.Index(&cfg.Info, "recap_link_by_family", vpack.FInt, vpack.FInt)

const (
	RecapPathPrefix       = "/recap/"
	recapTokenBytes       = 16
	maxRecapLinksPerEvent = 20
	maxRecapExpiryDays    = 365
	maxRecapRoutines      = 200
	maxRecapTextLength    = 300
)

var (
	ErrRecapEmpty          = errors.New("Choose at least one routine to include")
	ErrRecapTooLarge       = errors.New("That recap is larger than a link can hold")
	ErrRecapNotThisEvent   = errors.New("That recap includes results from another event")
	ErrRecapOutOfDate      = errors.New("These results changed since you opened them. Reload the page and try again.")
	ErrRecapExpiry         = errors.New("A recap link can last at most a year")
	ErrRecapLinkLimit      = errors.New("This event already has as many recap links as it can hold. Turn one off first.")
	ErrRecapLinkNotFound   = errors.New("Recap link not found")
	ErrRecapLinkNotAllowed = errors.New("Only the person who created this link or a family admin can turn it off")
)

func hashRecapToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func recapURL(token string) string {
	return siteRoot() + RecapPathPrefix + token
}

func GetRecapLinkById(tx *vbolt.Tx, id int) (link RecapLink) {
	vbolt.Read(tx, RecapLinkBkt, id, &link)
	return
}

func getEventRecapLinks(tx *vbolt.Tx, eventId int) []RecapLink {
	return readByTerm(tx, RecapLinkByEventIndex, RecapLinkBkt, eventId)
}

func findRecapLinkByToken(tx *vbolt.Tx, token string) (RecapLink, bool) {
	var id int
	vbolt.Read(tx, RecapLinkByHashBkt, hashRecapToken(token), &id)
	if id == 0 {
		return RecapLink{}, false
	}
	link := GetRecapLinkById(tx, id)
	return link, link.Id != 0
}

func (link RecapLink) liveAt(now time.Time) bool {
	return link.Id != 0 && (link.ExpiresAt.IsZero() || now.Before(link.ExpiresAt))
}

func writeRecapLinkTx(tx *vbolt.Tx, link *RecapLink) {
	vbolt.Write(tx, RecapLinkBkt, link.Id, link)
	vbolt.Write(tx, RecapLinkByHashBkt, link.TokenHash, &link.Id)
	vbolt.SetTargetSingleTerm(tx, RecapLinkByEventIndex, link.Id, link.EventId)
	vbolt.SetTargetSingleTerm(tx, RecapLinkByFamilyIndex, link.Id, link.FamilyId)
}

func deleteRecapLinkTx(tx *vbolt.Tx, link RecapLink) {
	vbolt.Delete(tx, RecapLinkBkt, link.Id)
	vbolt.Delete(tx, RecapLinkByHashBkt, link.TokenHash)
	vbolt.SetTargetSingleTerm(tx, RecapLinkByEventIndex, link.Id, -1)
	vbolt.SetTargetSingleTerm(tx, RecapLinkByFamilyIndex, link.Id, -1)
}

func deleteEventRecapLinksTx(tx *vbolt.Tx, eventId int) {
	for _, link := range getEventRecapLinks(tx, eventId) {
		deleteRecapLinkTx(tx, link)
	}
}

// RecapInput is the preview the sender approved. Entry and appearance ids, and
// each appearance's revision, let the server check that every included result
// belongs to this event and was current when the sender saw it; none of them
// are stored with the snapshot.
type RecapInput struct {
	EventName     string              `json:"eventName"`
	Subtitle      string              `json:"subtitle"`
	Place         string              `json:"place"`
	SnapshotLabel string              `json:"snapshotLabel"`
	Routines      []RecapRoutineInput `json:"routines"`
}

type RecapRoutineInput struct {
	EntryId      int                     `json:"entryId"`
	Name         string                  `json:"name"`
	Descriptors  string                  `json:"descriptors"`
	Roster       string                  `json:"roster"`
	Summary      string                  `json:"summary"`
	Performances []RecapPerformanceInput `json:"performances"`
}

type RecapPerformanceInput struct {
	AppearanceId int               `json:"appearanceId"`
	Revision     int               `json:"revision"`
	When         string            `json:"when"`
	Matchup      string            `json:"matchup"`
	Results      []PublishedResult `json:"results"`
}

type CreateRecapLinkRequest struct {
	EventId       int        `json:"eventId"`
	ExpiresInDays int        `json:"expiresInDays"`
	Recap         RecapInput `json:"recap"`
}

type RecapLinkView struct {
	Id            int        `json:"id"`
	CreatedAt     time.Time  `json:"createdAt"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	Expired       bool       `json:"expired"`
	CreatedByName string     `json:"createdByName"`
	CanRevoke     bool       `json:"canRevoke"`
	RoutineCount  int        `json:"routineCount"`
	SnapshotLabel string     `json:"snapshotLabel"`
}

type CreateRecapLinkResponse struct {
	Link RecapLinkView `json:"link"`
	URL  string        `json:"url"`
}

type ListRecapLinksRequest struct {
	EventId int `json:"eventId"`
}

type ListRecapLinksResponse struct {
	Links []RecapLinkView `json:"links"`
}

type RevokeRecapLinkRequest struct {
	Id int `json:"id"`
}
