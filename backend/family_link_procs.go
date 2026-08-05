package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterFamilyLinkMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListFamilyLinks)
	vbeam.RegisterProc(app, CreateFamilyLink)
	vbeam.RegisterProc(app, AcceptFamilyLink)
	vbeam.RegisterProc(app, UpdateFamilyLink)
	vbeam.RegisterProc(app, RevokeFamilyLink)
	vbeam.RegisterProc(app, GetPersonSharing)
	vbeam.RegisterProc(app, SharePersonWithFamily)
	vbeam.RegisterProc(app, UnsharePersonFromFamily)
}

// FamilyLinkView is one link as the caller's own family sees it. Direction is
// relative to the family that was asked about, because the same row reads as
// "we share with them" from one side and "they share with us" from the other,
// and the two carry different controls.
type FamilyLinkView struct {
	Id             int         `json:"id"`
	FromFamilyId   int         `json:"fromFamilyId"`
	FromFamilyName string      `json:"fromFamilyName"`
	ToFamilyId     int         `json:"toFamilyId"`
	ToFamilyName   string      `json:"toFamilyName"`
	Kind           string      `json:"kind"`
	Access         AccessLevel `json:"access"`
	Scopes         LinkScopes  `json:"scopes"`
	Status         LinkStatus  `json:"status"`
	CreatedAt      time.Time   `json:"createdAt"`
	// Outgoing means this family is the one sharing. Only the sharing side
	// chooses what a link carries; only the receiving side accepts it.
	Outgoing bool `json:"outgoing"`
	// SharedCount is how many of the sharing family's people are currently on
	// the receiving family's roster through this relationship.
	SharedCount int `json:"sharedCount"`
}

type ListFamilyLinksRequest struct {
	// FamilyId names which of the caller's families to report on. Zero means
	// all of them, which is what the settings page asks for.
	FamilyId int `json:"familyId,omitempty"`
}

type ListFamilyLinksResponse struct {
	Links []FamilyLinkView `json:"links"`
}

type CreateFamilyLinkRequest struct {
	// FamilyId is the family doing the sharing; zero means the caller's
	// primary family. The caller must be an admin of it, since this gives
	// something away.
	FamilyId int `json:"familyId,omitempty"`
	// InviteCode identifies the family being shared with. It is the same code
	// used to join a family, which is already a bearer secret for the strictly
	// greater power of becoming a member.
	InviteCode string     `json:"inviteCode"`
	Kind       string     `json:"kind,omitempty"`
	Scopes     LinkScopes `json:"scopes"`
}

type CreateFamilyLinkResponse struct {
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
	Link    FamilyLinkView `json:"link,omitempty"`
}

type FamilyLinkIdRequest struct {
	Id int `json:"id"`
}

type UpdateFamilyLinkRequest struct {
	Id     int        `json:"id"`
	Kind   string     `json:"kind,omitempty"`
	Scopes LinkScopes `json:"scopes"`
}

type FamilyLinkActionResponse struct {
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
	Link    FamilyLinkView `json:"link,omitempty"`
}

func linkView(tx *vbolt.Tx, link FamilyLink, forFamilyId int) FamilyLinkView {
	return FamilyLinkView{
		Id:             link.Id,
		FromFamilyId:   link.FromFamilyId,
		FromFamilyName: GetFamily(tx, link.FromFamilyId).Name,
		ToFamilyId:     link.ToFamilyId,
		ToFamilyName:   GetFamily(tx, link.ToFamilyId).Name,
		Kind:           link.Kind,
		Access:         link.Access,
		Scopes:         linkScopesFromMask(link.Scopes),
		Status:         link.Status,
		CreatedAt:      link.CreatedAt,
		Outgoing:       link.FromFamilyId == forFamilyId,
		SharedCount:    countSharedThroughLink(tx, link),
	}
}

// countSharedThroughLink counts the sharing family's people who currently sit
// on the receiving family's roster.
func countSharedThroughLink(tx *vbolt.Tx, link FamilyLink) (count int) {
	for _, row := range GetFamilyRoster(tx, link.ToFamilyId) {
		person := GetPersonById(tx, row.PersonId)
		if person.Id != 0 && person.FamilyId == link.FromFamilyId {
			count++
		}
	}
	return
}

func ListFamilyLinks(ctx *vbeam.Context, req ListFamilyLinksRequest) (resp ListFamilyLinksResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	familyIds := familiesVisibleTo(ctx.Tx, user)
	if req.FamilyId != 0 {
		if err = RequireFamilyAccess(ctx.Tx, user, req.FamilyId, AccessView); err != nil {
			return
		}
		familyIds = []int{req.FamilyId}
	}

	resp.Links = []FamilyLinkView{}
	seen := make(map[int]bool)
	for _, familyId := range familyIds {
		links := append(GetLinksFromFamily(ctx.Tx, familyId), GetLinksToFamily(ctx.Tx, familyId)...)
		for _, link := range links {
			if link.Id == 0 || seen[link.Id] || link.Status == LinkRevoked {
				continue
			}
			seen[link.Id] = true
			resp.Links = append(resp.Links, linkView(ctx.Tx, link, familyId))
		}
	}
	return
}

func CreateFamilyLink(ctx *vbeam.Context, req CreateFamilyLinkRequest) (resp CreateFamilyLinkResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Sharing gives something away, so it takes admin rights in the family
	// whose content is being shared.
	fromFamilyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessAdmin)
	if err != nil {
		return
	}

	target := GetFamilyByInviteCode(ctx.Tx, req.InviteCode)
	if target.Id == 0 {
		resp.Error = "Invalid invite code"
		return
	}
	if target.Id == fromFamilyId {
		resp.Error = ErrLinkToSelf.Error()
		return
	}
	if _, exists := FindFamilyLink(ctx.Tx, fromFamilyId, target.Id); exists {
		resp.Error = ErrLinkExists.Error()
		return
	}

	scopes := normalizeLinkScopes(req.Scopes)
	mask := scopes.ToMask()
	if mask == 0 {
		resp.Error = "A link has to share something"
		return
	}

	vbeam.UseWriteTx(ctx)
	link := createFamilyLinkTx(ctx.Tx, fromFamilyId, target.Id, normalizeLinkKind(req.Kind), AccessView, mask)
	view := linkView(ctx.Tx, link, fromFamilyId)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Link = view
	return
}

func AcceptFamilyLink(ctx *vbeam.Context, req FamilyLinkIdRequest) (resp FamilyLinkActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	link := GetFamilyLinkById(ctx.Tx, req.Id)
	if link.Id == 0 {
		err = ErrLinkNotFound
		return
	}
	// Only the receiving family accepts: the offer is theirs to take or leave,
	// and accepting is what puts their roster in reach of the other household.
	if err = RequireFamilyAccess(ctx.Tx, user, link.ToFamilyId, AccessAdmin); err != nil {
		return
	}
	if link.Status == LinkRevoked {
		resp.Error = "That link has been revoked"
		return
	}

	vbeam.UseWriteTx(ctx)
	link.Status = LinkAccepted
	writeFamilyLinkTx(ctx.Tx, link)
	view := linkView(ctx.Tx, link, link.ToFamilyId)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Link = view
	return
}

func UpdateFamilyLink(ctx *vbeam.Context, req UpdateFamilyLinkRequest) (resp FamilyLinkActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	link := GetFamilyLinkById(ctx.Tx, req.Id)
	if link.Id == 0 {
		err = ErrLinkNotFound
		return
	}
	// What a link carries is the sharing family's decision alone. The receiving
	// family can decline it or hand it back, but it cannot widen it.
	if err = RequireFamilyAccess(ctx.Tx, user, link.FromFamilyId, AccessAdmin); err != nil {
		return
	}
	if link.Status == LinkRevoked {
		resp.Error = "That link has been revoked"
		return
	}

	scopes := normalizeLinkScopes(req.Scopes)
	mask := scopes.ToMask()
	if mask == 0 {
		resp.Error = "A link has to share something"
		return
	}

	vbeam.UseWriteTx(ctx)
	link.Kind = normalizeLinkKind(req.Kind)
	link.Scopes = mask
	writeFamilyLinkTx(ctx.Tx, link)
	// Narrowing a link can strip the People scope, which is what put the shared
	// people on the other roster in the first place. Leaving those rows behind
	// would show the receiving family a list of names with nothing behind them.
	if !scopes.People {
		unshareAllThroughLinkTx(ctx.Tx, link)
	}
	view := linkView(ctx.Tx, link, link.FromFamilyId)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Link = view
	return
}

func RevokeFamilyLink(ctx *vbeam.Context, req FamilyLinkIdRequest) (resp FamilyLinkActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	link := GetFamilyLinkById(ctx.Tx, req.Id)
	if link.Id == 0 {
		err = ErrLinkNotFound
		return
	}
	// Either side can end the relationship: the sharing family by withdrawing
	// the offer, the receiving family by declining to hold it.
	fromErr := RequireFamilyAccess(ctx.Tx, user, link.FromFamilyId, AccessAdmin)
	toErr := RequireFamilyAccess(ctx.Tx, user, link.ToFamilyId, AccessAdmin)
	if fromErr != nil && toErr != nil {
		err = ErrFamilyAccessDenied
		return
	}

	vbeam.UseWriteTx(ctx)
	link.Status = LinkRevoked
	writeFamilyLinkTx(ctx.Tx, link)
	// The shared people go with it. A revoked link grants nothing, so the rows
	// would otherwise leave unreachable names on the receiving family's roster.
	unshareAllThroughLinkTx(ctx.Tx, link)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

// unshareAllThroughLinkTx takes the sharing family's people back off the
// receiving family's roster. Only rows placed by this relationship are removed:
// a person's home roster is never touched, and neither is a roster row the
// receiving family owns because the person is homed there.
func unshareAllThroughLinkTx(tx *vbolt.Tx, link FamilyLink) {
	for _, row := range GetFamilyRoster(tx, link.ToFamilyId) {
		person := GetPersonById(tx, row.PersonId)
		if person.Id == 0 || person.FamilyId != link.FromFamilyId {
			continue
		}
		deletePersonFamilyTx(tx, row)
	}
}

// SharedRosterRef is one family a person appears on beyond their own household.
type SharedRosterRef struct {
	FamilyId     int        `json:"familyId"`
	FamilyName   string     `json:"familyName"`
	Role         PersonType `json:"role"`
	Relationship string     `json:"relationship,omitempty"`
}

// ShareTargetRef is a family this person could be shared into: one with an
// accepted link from their home family that carries people.
type ShareTargetRef struct {
	FamilyId   int    `json:"familyId"`
	FamilyName string `json:"familyName"`
	Kind       string `json:"kind,omitempty"`
}

type GetPersonSharingRequest struct {
	PersonId int `json:"personId"`
}

type GetPersonSharingResponse struct {
	PersonId     int               `json:"personId"`
	HomeFamilyId int               `json:"homeFamilyId"`
	SharedWith   []SharedRosterRef `json:"sharedWith"`
	// CanShare is empty when the home family has no accepted outbound link, in
	// which case the answer to "who may I share into" is "nobody yet" rather
	// than "any family you happen to belong to".
	CanShare []ShareTargetRef `json:"canShare"`
	// Manageable is false for a viewer who can see the person but does not
	// administer the household that owns them.
	Manageable bool `json:"manageable"`
}

type SharePersonRequest struct {
	PersonId int    `json:"personId"`
	FamilyId int    `json:"familyId"`
	Role     int    `json:"role,omitempty"`
	Kind     string `json:"relationship,omitempty"`
}

type UnsharePersonRequest struct {
	PersonId int `json:"personId"`
	FamilyId int `json:"familyId"`
}

type PersonSharingActionResponse struct {
	Success bool                     `json:"success"`
	Error   string                   `json:"error,omitempty"`
	Sharing GetPersonSharingResponse `json:"sharing,omitempty"`
}

func personSharing(tx *vbolt.Tx, user User, person Person) GetPersonSharingResponse {
	resp := GetPersonSharingResponse{
		PersonId:     person.Id,
		HomeFamilyId: person.FamilyId,
		SharedWith:   []SharedRosterRef{},
		CanShare:     []ShareTargetRef{},
		Manageable:   CanAccessFamily(tx, user, person.FamilyId, AccessAdmin),
	}

	onRoster := make(map[int]bool)
	for _, row := range GetPersonFamilies(tx, person.Id) {
		onRoster[row.FamilyId] = true
		if row.FamilyId == person.FamilyId {
			continue // the home roster is not a share
		}
		resp.SharedWith = append(resp.SharedWith, SharedRosterRef{
			FamilyId:     row.FamilyId,
			FamilyName:   GetFamily(tx, row.FamilyId).Name,
			Role:         row.Role,
			Relationship: row.Relationship,
		})
	}

	if !resp.Manageable {
		return resp
	}
	// Which families a person may be placed into is a link question, not a
	// membership one: the home family must have offered its people to them and
	// they must have accepted.
	for _, link := range GetLinksFromFamily(tx, person.FamilyId) {
		if link.Status != LinkAccepted || !link.HasScope(ScopePeople) || onRoster[link.ToFamilyId] {
			continue
		}
		resp.CanShare = append(resp.CanShare, ShareTargetRef{
			FamilyId:   link.ToFamilyId,
			FamilyName: GetFamily(tx, link.ToFamilyId).Name,
			Kind:       link.Kind,
		})
	}
	return resp
}

func GetPersonSharing(ctx *vbeam.Context, req GetPersonSharingRequest) (resp GetPersonSharingResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopePeople, AccessView) {
		err = errors.New("Person not found or not in your family")
		return
	}
	resp = personSharing(ctx.Tx, user, person)
	return
}

func SharePersonWithFamily(ctx *vbeam.Context, req SharePersonRequest) (resp PersonSharingActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 {
		err = errors.New("Person not found")
		return
	}
	// Sharing a person out is the home family's decision, so it takes admin
	// rights there — not in the family receiving them.
	if err = RequireFamilyAccess(ctx.Tx, user, person.FamilyId, AccessAdmin); err != nil {
		return
	}
	if req.FamilyId == person.FamilyId {
		resp.Error = "That is already this person's own family"
		return
	}
	if !CanShareIntoFamily(ctx.Tx, person.FamilyId, req.FamilyId) {
		resp.Error = "Those families are not linked, or the link does not share people"
		return
	}

	role := PersonType(req.Role)
	if req.Role == 0 && person.Type != 0 {
		role = person.Type
	}

	vbeam.UseWriteTx(ctx)
	row := EnsurePersonFamilyTx(ctx.Tx, person.Id, req.FamilyId, role)
	if relationship := normalizeLinkKind(req.Kind); relationship != "" && row.Relationship != relationship {
		row.Relationship = relationship
		vbolt.Write(ctx.Tx, PersonFamilyBkt, row.Id, &row)
	}
	sharing := personSharing(ctx.Tx, user, person)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Sharing = sharing
	return
}

func UnsharePersonFromFamily(ctx *vbeam.Context, req UnsharePersonRequest) (resp PersonSharingActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 {
		err = errors.New("Person not found")
		return
	}
	// Either household can end a share: the owner by withdrawing the person,
	// the host by taking them off its own roster.
	ownerErr := RequireFamilyAccess(ctx.Tx, user, person.FamilyId, AccessAdmin)
	hostErr := RequireFamilyAccess(ctx.Tx, user, req.FamilyId, AccessAdmin)
	if ownerErr != nil && hostErr != nil {
		err = ErrFamilyAccessDenied
		return
	}

	vbeam.UseWriteTx(ctx)
	if err = RemovePersonFromFamilyTx(ctx.Tx, person.Id, req.FamilyId); err != nil {
		return
	}
	sharing := personSharing(ctx.Tx, user, person)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Sharing = sharing
	return
}
