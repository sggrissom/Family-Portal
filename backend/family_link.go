package backend

import (
	"errors"
	"family/cfg"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

type LinkStatus int

const (
	LinkPending LinkStatus = iota
	LinkAccepted
	LinkRevoked
)

type LinkScope int

const (
	ScopeFamily LinkScope = iota
	ScopePeople
	ScopeMilestones
	ScopePhotos
	ScopeGrowth
	ScopeActivities
)

func (scope LinkScope) bit() int { return 1 << uint(scope) }

const MaxLinkAccess = AccessView

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

var FamilyLinkByFromIndex = vbolt.Index(&cfg.Info, "family_link_by_from", vpack.FInt, vpack.FInt)

var FamilyLinkByToIndex = vbolt.Index(&cfg.Info, "family_link_by_to", vpack.FInt, vpack.FInt)

var ErrLinkNotFound = errors.New("Family link not found")
var ErrLinkToSelf = errors.New("A family cannot be linked to itself")
var ErrLinkExists = errors.New("These families are already linked in that direction")

func (link FamilyLink) HasScope(scope LinkScope) bool {
	return link.Scopes&scope.bit() != 0
}

func GetLinksFromFamily(tx *vbolt.Tx, familyId int) (links []FamilyLink) {
	if familyId == 0 {
		return
	}
	var ids []int
	vbolt.ReadTermTargets(tx, FamilyLinkByFromIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyLinkBkt, ids, &links)
	return
}

func GetLinksToFamily(tx *vbolt.Tx, familyId int) (links []FamilyLink) {
	if familyId == 0 {
		return
	}
	var ids []int
	vbolt.ReadTermTargets(tx, FamilyLinkByToIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyLinkBkt, ids, &links)
	return
}

func FindFamilyLink(tx *vbolt.Tx, fromFamilyId int, toFamilyId int) (FamilyLink, bool) {
	for _, link := range GetLinksFromFamily(tx, fromFamilyId) {
		if link.ToFamilyId == toFamilyId && link.Status != LinkRevoked {
			return link, true
		}
	}
	return FamilyLink{}, false
}

func GetFamilyLinkById(tx *vbolt.Tx, linkId int) (link FamilyLink) {
	vbolt.Read(tx, FamilyLinkBkt, linkId, &link)
	return
}

func linkGrants(tx *vbolt.Tx, actingFamilyId int, familyId int, scope LinkScope) AccessLevel {
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

type LinkScopes struct {
	People     bool `json:"people"`
	Milestones bool `json:"milestones"`
	Photos     bool `json:"photos"`
	Growth     bool `json:"growth"`
	Activities bool `json:"activities"`
}

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
	if scopes.Activities {
		mask |= ScopeActivities.bit()
	}
	return mask
}

func linkScopesFromMask(mask int) LinkScopes {
	return LinkScopes{
		People:     mask&ScopePeople.bit() != 0,
		Milestones: mask&ScopeMilestones.bit() != 0,
		Photos:     mask&ScopePhotos.bit() != 0,
		Growth:     mask&ScopeGrowth.bit() != 0,
		Activities: mask&ScopeActivities.bit() != 0,
	}
}

func normalizeLinkScopes(scopes LinkScopes) LinkScopes {
	if scopes.Milestones || scopes.Photos || scopes.Growth || scopes.Activities {
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
