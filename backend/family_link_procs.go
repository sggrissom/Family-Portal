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
	Outgoing       bool        `json:"outgoing"`
	SharedCount    int         `json:"sharedCount"`
}

type ListFamilyLinksRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type ListFamilyLinksResponse struct {
	Links []FamilyLinkView `json:"links"`
}

type CreateFamilyLinkRequest struct {
	FamilyId   int        `json:"familyId,omitempty"`
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
	fromErr := RequireFamilyAccess(ctx.Tx, user, link.FromFamilyId, AccessAdmin)
	toErr := RequireFamilyAccess(ctx.Tx, user, link.ToFamilyId, AccessAdmin)
	if fromErr != nil && toErr != nil {
		err = ErrFamilyAccessDenied
		return
	}

	vbeam.UseWriteTx(ctx)
	link.Status = LinkRevoked
	writeFamilyLinkTx(ctx.Tx, link)
	unshareAllThroughLinkTx(ctx.Tx, link)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

func unshareAllThroughLinkTx(tx *vbolt.Tx, link FamilyLink) {
	for _, row := range GetFamilyRoster(tx, link.ToFamilyId) {
		person := GetPersonById(tx, row.PersonId)
		if person.Id == 0 || person.FamilyId != link.FromFamilyId {
			continue
		}
		deletePersonFamilyTx(tx, row)
	}
}

type SharedRosterRef struct {
	FamilyId     int        `json:"familyId"`
	FamilyName   string     `json:"familyName"`
	Role         PersonType `json:"role"`
	Relationship string     `json:"relationship,omitempty"`
}

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
	CanShare     []ShareTargetRef  `json:"canShare"`
	Manageable   bool              `json:"manageable"`
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
			continue
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
