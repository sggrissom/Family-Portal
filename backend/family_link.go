package backend

import (
	"errors"
	"family/cfg"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// LinkStatus is where a link sits in the invite/accept lifecycle. A link grants
// nothing until it is accepted, and nothing again once it is revoked.
type LinkStatus int

const (
	LinkPending LinkStatus = iota
	LinkAccepted
	LinkRevoked
)

// LinkScope names one kind of content a link can share. Access alone cannot
// answer "may the grandparents see this" — a family that wants to share
// milestones and photos rarely wants to share weight measurements or the family
// chat — so what a link opens up is encoded per entity rather than inferred
// from the AccessLevel ladder.
//
// ScopeFamily is the structural scope: family settings, invite codes, tags,
// merges, deletions. It is deliberately not grantable by a link, which is what
// keeps a link from turning into a back door into membership.
//
// There is no chat scope. Every other scope hangs off a person, so a link can
// share one child without sharing the household; chat is a single room per
// family with no per-person dimension, and ChatHub still subscribes each client
// to exactly one room (see Stage 6 of docs/multi-family-plan.md). Offering it
// here would mean sharing a whole conversation on a model that cannot yet
// deliver it live.
type LinkScope int

const (
	ScopeFamily LinkScope = iota
	ScopePeople
	ScopeMilestones
	ScopePhotos
	ScopeGrowth
)

// bit is this scope's position in a FamilyLink.Scopes mask.
func (scope LinkScope) bit() int { return 1 << uint(scope) }

// MaxLinkAccess caps what a link may grant. Links are read-only in this stage:
// the ladder above AccessView exists so a later stage can let a linked family
// contribute, but every write path still requires membership today. Because the
// cap is enforced here rather than at each call site, a write check simply fails
// to be satisfied by a link.
const MaxLinkAccess = AccessView

// FamilyLink relates two families in one direction: ToFamily may do Access,
// within Scopes, in FromFamily. Direction matters — "grandparents may see the
// grandkids' milestones" must not imply the reverse — so reciprocal access is
// two rows, not one symmetric one.
type FamilyLink struct {
	Id           int         `json:"id"`
	FromFamilyId int         `json:"fromFamilyId"`
	ToFamilyId   int         `json:"toFamilyId"`
	Kind         string      `json:"kind"`
	Access       AccessLevel `json:"access"`
	Scopes       int         `json:"scopes"`
	Status       LinkStatus  `json:"status"`
	CreatedAt    time.Time   `json:"createdAt"`
}

func PackFamilyLink(self *FamilyLink, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FromFamilyId, buf)
	vpack.Int(&self.ToFamilyId, buf)
	vpack.String(&self.Kind, buf)
	vpack.IntEnum(&self.Access, buf)
	vpack.Int(&self.Scopes, buf)
	vpack.IntEnum(&self.Status, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var FamilyLinkBkt = vbolt.Bucket(&cfg.Info, "family_link", vpack.FInt, PackFamilyLink)

// FamilyLinkByFromIndex: term = from_family_id, target = link_id
var FamilyLinkByFromIndex = vbolt.Index(&cfg.Info, "family_link_by_from", vpack.FInt, vpack.FInt)

// FamilyLinkByToIndex: term = to_family_id, target = link_id
var FamilyLinkByToIndex = vbolt.Index(&cfg.Info, "family_link_by_to", vpack.FInt, vpack.FInt)

var ErrLinkNotFound = errors.New("Family link not found")
var ErrLinkToSelf = errors.New("A family cannot be linked to itself")
var ErrLinkExists = errors.New("These families are already linked in that direction")

// HasScope reports whether the link shares this kind of content.
func (link FamilyLink) HasScope(scope LinkScope) bool {
	return link.Scopes&scope.bit() != 0
}

// GetLinksFromFamily returns the links where this family is the one sharing.
func GetLinksFromFamily(tx *vbolt.Tx, familyId int) (links []FamilyLink) {
	if familyId == 0 {
		return
	}
	var ids []int
	vbolt.ReadTermTargets(tx, FamilyLinkByFromIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyLinkBkt, ids, &links)
	return
}

// GetLinksToFamily returns the links where this family is the one receiving.
func GetLinksToFamily(tx *vbolt.Tx, familyId int) (links []FamilyLink) {
	if familyId == 0 {
		return
	}
	var ids []int
	vbolt.ReadTermTargets(tx, FamilyLinkByToIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyLinkBkt, ids, &links)
	return
}

// FindFamilyLink looks up the live link in one direction, if there is one. A
// revoked link is not live: revoking then re-inviting reuses the same row.
func FindFamilyLink(tx *vbolt.Tx, fromFamilyId int, toFamilyId int) (FamilyLink, bool) {
	for _, link := range GetLinksFromFamily(tx, fromFamilyId) {
		if link.ToFamilyId == toFamilyId && link.Status != LinkRevoked {
			return link, true
		}
	}
	return FamilyLink{}, false
}

// GetFamilyLinkById reads one link by id.
func GetFamilyLinkById(tx *vbolt.Tx, linkId int) (link FamilyLink) {
	vbolt.Read(tx, FamilyLinkBkt, linkId, &link)
	return
}

// linkGrants reports what a caller acting in actingFamilyId may do inside
// familyId by way of an accepted link, for one kind of content.
//
// Traversal is one hop and only one hop: the only rows consulted are those
// whose ToFamilyId is the acting family and whose FromFamilyId is the family
// being reached. Nothing here follows familyId's own links, so A→B and B→C
// never combine into A→C. There is no recursion to bound because the lookup
// never takes a second step.
func linkGrants(tx *vbolt.Tx, actingFamilyId int, familyId int, scope LinkScope) AccessLevel {
	// Structural access is membership-only. A link shares content; it never
	// makes the receiving family an administrator of the sharing one.
	if scope == ScopeFamily {
		return AccessNone
	}
	granted := AccessNone
	for _, link := range GetLinksToFamily(tx, actingFamilyId) {
		if link.FromFamilyId != familyId || link.Status != LinkAccepted {
			continue
		}
		if !link.HasScope(scope) {
			continue
		}
		if link.Access > granted {
			granted = link.Access
		}
	}
	if granted > MaxLinkAccess {
		granted = MaxLinkAccess
	}
	return granted
}

// CanShareIntoFamily reports whether ownerFamilyId may place its people on
// targetFamilyId's roster: there must be an accepted link sharing the owner's
// people with the target. Sharing a person is the owner giving something away,
// so the link that authorizes it is the owner's outbound one.
func CanShareIntoFamily(tx *vbolt.Tx, ownerFamilyId int, targetFamilyId int) bool {
	if ownerFamilyId == 0 || targetFamilyId == 0 || ownerFamilyId == targetFamilyId {
		return false
	}
	for _, link := range GetLinksFromFamily(tx, ownerFamilyId) {
		if link.ToFamilyId != targetFamilyId || link.Status != LinkAccepted {
			continue
		}
		if link.HasScope(ScopePeople) {
			return true
		}
	}
	return false
}

// createFamilyLinkTx records a pending link. Callers check authorization and
// that no live link already exists in this direction.
func createFamilyLinkTx(tx *vbolt.Tx, fromFamilyId int, toFamilyId int, kind string, access AccessLevel, scopes int) FamilyLink {
	link := FamilyLink{
		Id:           vbolt.NextIntId(tx, FamilyLinkBkt),
		FromFamilyId: fromFamilyId,
		ToFamilyId:   toFamilyId,
		Kind:         kind,
		Access:       clampLinkAccess(access),
		Scopes:       scopes,
		Status:       LinkPending,
		CreatedAt:    time.Now(),
	}
	writeFamilyLinkTx(tx, link)
	return link
}

func writeFamilyLinkTx(tx *vbolt.Tx, link FamilyLink) {
	vbolt.Write(tx, FamilyLinkBkt, link.Id, &link)
	vbolt.SetTargetSingleTerm(tx, FamilyLinkByFromIndex, link.Id, link.FromFamilyId)
	vbolt.SetTargetSingleTerm(tx, FamilyLinkByToIndex, link.Id, link.ToFamilyId)
}

func clampLinkAccess(access AccessLevel) AccessLevel {
	if access < AccessView {
		return AccessView
	}
	if access > MaxLinkAccess {
		return MaxLinkAccess
	}
	return access
}

// LinkScopes is the per-entity share list in the shape the API speaks. The
// stored form is a bitmask, which is compact but unreadable on the wire and in
// a UI; this is the same information with names on it.
type LinkScopes struct {
	People     bool `json:"people"`
	Milestones bool `json:"milestones"`
	Photos     bool `json:"photos"`
	Growth     bool `json:"growth"`
}

// DefaultLinkScopes is what a new link shares unless the granting family says
// otherwise: the people put on the other family's roster, and their milestones
// and photos. Growth measurements are medical data and stay off until they are
// deliberately turned on.
func DefaultLinkScopes() LinkScopes {
	return LinkScopes{People: true, Milestones: true, Photos: true}
}

func (scopes LinkScopes) ToMask() int {
	mask := 0
	if scopes.People {
		mask |= ScopePeople.bit()
	}
	if scopes.Milestones {
		mask |= ScopeMilestones.bit()
	}
	if scopes.Photos {
		mask |= ScopePhotos.bit()
	}
	if scopes.Growth {
		mask |= ScopeGrowth.bit()
	}
	return mask
}

func linkScopesFromMask(mask int) LinkScopes {
	return LinkScopes{
		People:     mask&ScopePeople.bit() != 0,
		Milestones: mask&ScopeMilestones.bit() != 0,
		Photos:     mask&ScopePhotos.bit() != 0,
		Growth:     mask&ScopeGrowth.bit() != 0,
	}
}

// normalizeLinkScopes keeps the mask meaningful. Sharing milestones, photos or
// measurements of people the other family cannot see is not a coherent state —
// every one of those reads resolves through a person — so People is implied by
// any of them. An empty mask shares nothing and is rejected by the callers.
func normalizeLinkScopes(scopes LinkScopes) LinkScopes {
	if scopes.Milestones || scopes.Photos || scopes.Growth {
		scopes.People = true
	}
	return scopes
}

func normalizeLinkKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if len(kind) > 40 {
		kind = kind[:40]
	}
	return kind
}
