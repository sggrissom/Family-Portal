package backend

import (
	"errors"
	"family/cfg"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterRelationMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetPersonRelations)
	vbeam.RegisterProc(app, GetRelationLabels)
	vbeam.RegisterProc(app, AddRelation)
	vbeam.RegisterProc(app, RemoveRelation)
}

type RelationKind int

const (
	RelationParent RelationKind = iota
	RelationSibling
	RelationPartner
)

type Relation struct {
	Id     int          `json:"id"`
	FromId int          `json:"fromId"`
	ToId   int          `json:"toId"`
	Kind   RelationKind `json:"kind"`
}

func PackRelation(self *Relation, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FromId, buf)
	vpack.Int(&self.ToId, buf)
	vpack.IntEnum(&self.Kind, buf)
}

var RelationBkt = vbolt.Bucket(&cfg.Info, "relation", vpack.FInt, PackRelation)

var RelationByPersonIndex = vbolt.Index(&cfg.Info, "relation_by_person", vpack.FInt, vpack.FInt)

var ErrRelationToSelf = errors.New("A person cannot be related to themselves")

// StatedRelation is how a relationship is phrased when entered: it names what
// the new person is to the anchor, so the wire value carries direction.
type StatedRelation int

const (
	StatedNone StatedRelation = iota
	StatedChild
	StatedParent
	StatedSibling
	StatedPartner
)

func (s StatedRelation) valid() bool {
	return s >= StatedNone && s <= StatedPartner
}

func (s StatedRelation) edge(personId int, anchorId int) (Relation, bool) {
	switch s {
	case StatedChild:
		return Relation{FromId: anchorId, ToId: personId, Kind: RelationParent}, true
	case StatedParent:
		return Relation{FromId: personId, ToId: anchorId, Kind: RelationParent}, true
	case StatedSibling:
		return Relation{FromId: personId, ToId: anchorId, Kind: RelationSibling}, true
	case StatedPartner:
		return Relation{FromId: personId, ToId: anchorId, Kind: RelationPartner}, true
	}
	return Relation{}, false
}

func GetPersonRelationsTx(tx *vbolt.Tx, personId int) (rows []Relation) {
	var ids []int
	vbolt.ReadTermTargets(tx, RelationByPersonIndex, personId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, RelationBkt, ids, &rows)
	return
}

func findRelationTx(tx *vbolt.Tx, rel Relation) (Relation, bool) {
	for _, row := range GetPersonRelationsTx(tx, rel.FromId) {
		if row.Kind != rel.Kind {
			continue
		}
		if row.FromId == rel.FromId && row.ToId == rel.ToId {
			return row, true
		}
		// Sibling and partner edges are symmetric, so a stored row in either
		// direction already states the relationship.
		if rel.Kind != RelationParent && row.FromId == rel.ToId && row.ToId == rel.FromId {
			return row, true
		}
	}
	return Relation{}, false
}

func AddRelationTx(tx *vbolt.Tx, rel Relation) (Relation, error) {
	if rel.FromId == 0 || rel.ToId == 0 {
		return Relation{}, errors.New("Both people are required")
	}
	if rel.FromId == rel.ToId {
		return Relation{}, ErrRelationToSelf
	}
	if existing, found := findRelationTx(tx, rel); found {
		return existing, nil
	}
	rel.Id = vbolt.NextIntId(tx, RelationBkt)
	vbolt.Write(tx, RelationBkt, rel.Id, &rel)
	vbolt.SetTargetTermsPlain(tx, RelationByPersonIndex, rel.Id, []int{rel.FromId, rel.ToId})
	return rel, nil
}

func deleteRelationTx(tx *vbolt.Tx, rel Relation) {
	vbolt.Delete(tx, RelationBkt, rel.Id)
	vbolt.SetTargetTermsPlain(tx, RelationByPersonIndex, rel.Id, nil)
}

func deletePersonRelationsTx(tx *vbolt.Tx, personId int) {
	for _, row := range GetPersonRelationsTx(tx, personId) {
		deleteRelationTx(tx, row)
	}
}

func movePersonRelationsTx(tx *vbolt.Tx, fromPersonId int, toPersonId int) {
	for _, row := range GetPersonRelationsTx(tx, fromPersonId) {
		moved := row
		if moved.FromId == fromPersonId {
			moved.FromId = toPersonId
		}
		if moved.ToId == fromPersonId {
			moved.ToId = toPersonId
		}
		deleteRelationTx(tx, row)
		if moved.FromId != moved.ToId {
			AddRelationTx(tx, Relation{FromId: moved.FromId, ToId: moved.ToId, Kind: moved.Kind})
		}
	}
}

func parentsOf(tx *vbolt.Tx, personId int) (ids []int) {
	for _, row := range GetPersonRelationsTx(tx, personId) {
		if row.Kind == RelationParent && row.ToId == personId {
			ids = append(ids, row.FromId)
		}
	}
	return
}

func childrenOf(tx *vbolt.Tx, personId int) (ids []int) {
	for _, row := range GetPersonRelationsTx(tx, personId) {
		if row.Kind == RelationParent && row.FromId == personId {
			ids = append(ids, row.ToId)
		}
	}
	return
}

func statedPeers(tx *vbolt.Tx, personId int, kind RelationKind) (ids []int) {
	for _, row := range GetPersonRelationsTx(tx, personId) {
		if row.Kind != kind {
			continue
		}
		if row.FromId == personId {
			ids = append(ids, row.ToId)
		} else if row.ToId == personId {
			ids = append(ids, row.FromId)
		}
	}
	return
}

// applyStatedRelationTx writes one stated relation from person to each anchor.
// Anchors the caller cannot see, and the person themselves, are skipped rather
// than failing the whole save.
func applyStatedRelationTx(tx *vbolt.Tx, user User, personId int, stated StatedRelation, anchorIds []int) error {
	done := make(map[int]bool)
	for _, anchorId := range anchorIds {
		if anchorId == 0 || anchorId == personId || done[anchorId] {
			continue
		}
		done[anchorId] = true
		anchor := GetPersonById(tx, anchorId)
		if anchor.Id == 0 || !CanAccessPerson(tx, user, anchor, ScopePeople, AccessView) {
			return ErrPersonNotFound
		}
		edge, ok := stated.edge(personId, anchor.Id)
		if !ok {
			continue
		}
		if _, err := AddRelationTx(tx, edge); err != nil {
			return err
		}
	}
	return nil
}

func partnersOf(tx *vbolt.Tx, personId int) []int {
	return statedPeers(tx, personId, RelationPartner)
}

// SiblingsOf returns siblings stated directly plus those implied by a shared parent.
func SiblingsOf(tx *vbolt.Tx, personId int) []int {
	seen := map[int]bool{personId: true}
	var ids []int
	add := func(id int) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range statedPeers(tx, personId, RelationSibling) {
		add(id)
	}
	for _, parentId := range parentsOf(tx, personId) {
		for _, id := range childrenOf(tx, parentId) {
			add(id)
		}
	}
	return ids
}

// relationsAmong returns the stored edges whose endpoints are both in people,
// deduplicated, so a client holding that list can reason about the graph.
func relationsAmong(tx *vbolt.Tx, people []Person) []Relation {
	visible := make(map[int]bool, len(people))
	for _, person := range people {
		visible[person.Id] = true
	}
	seen := make(map[int]bool)
	rows := []Relation{}
	for _, person := range people {
		for _, row := range GetPersonRelationsTx(tx, person.Id) {
			if seen[row.Id] || !visible[row.FromId] || !visible[row.ToId] {
				continue
			}
			seen[row.Id] = true
			rows = append(rows, row)
		}
	}
	return rows
}

func containsId(ids []int, id int) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

type GetPersonRelationsRequest struct {
	PersonId int `json:"personId"`
}

type RelationView struct {
	Id         int    `json:"id"`
	PersonId   int    `json:"personId"`
	PersonName string `json:"personName"`
	Label      string `json:"label"`
	Stored     bool   `json:"stored"`
}

type GetPersonRelationsResponse struct {
	PersonId   int            `json:"personId"`
	Relations  []RelationView `json:"relations"`
	Manageable bool           `json:"manageable"`
}

type GetRelationLabelsRequest struct {
	SubjectId int `json:"subjectId"`
}

type RelationLabelEntry struct {
	PersonId int    `json:"personId"`
	Label    string `json:"label"`
	Group    string `json:"group"`
}

type GetRelationLabelsResponse struct {
	SubjectId int                  `json:"subjectId"`
	Labels    []RelationLabelEntry `json:"labels"`
}

type AddRelationRequest struct {
	PersonId int            `json:"personId"`
	AnchorId int            `json:"anchorId"`
	Stated   StatedRelation `json:"stated"`
	// AdditionalAnchorIds states the same relation against more people in one
	// call, so "child of Steven and Ruth" is a single save.
	AdditionalAnchorIds []int `json:"additionalAnchorIds,omitempty"`
}

type RemoveRelationRequest struct {
	RelationId int `json:"relationId"`
}

type RelationActionResponse struct {
	Success   bool                       `json:"success"`
	Error     string                     `json:"error,omitempty"`
	Relations GetPersonRelationsResponse `json:"relations,omitempty"`
}

// personRelations lists everyone related to person: the edges actually stored
// first, then everyone else the relationship graph implies. Only stored rows
// carry an Id, since only those can be removed.
func personRelations(tx *vbolt.Tx, user User, person Person) GetPersonRelationsResponse {
	resp := GetPersonRelationsResponse{
		PersonId:   person.Id,
		Relations:  []RelationView{},
		Manageable: CanAccessFamily(tx, user, person.FamilyId, AccessContribute),
	}
	stated := make(map[int]bool)
	for _, row := range GetPersonRelationsTx(tx, person.Id) {
		otherId := row.ToId
		if otherId == person.Id {
			otherId = row.FromId
		}
		other := GetPersonById(tx, otherId)
		if other.Id == 0 || !CanAccessPerson(tx, user, other, ScopePeople, AccessView) {
			continue
		}
		stated[other.Id] = true
		resp.Relations = append(resp.Relations, RelationView{
			Id:         row.Id,
			PersonId:   other.Id,
			PersonName: other.Name,
			Label:      RelationLabel(tx, person, other),
			Stored:     true,
		})
	}
	for _, other := range GetVisiblePeople(tx, user) {
		if other.Id == person.Id || stated[other.Id] {
			continue
		}
		label := RelationLabel(tx, person, other)
		if label == "" {
			continue
		}
		resp.Relations = append(resp.Relations, RelationView{
			PersonId:   other.Id,
			PersonName: other.Name,
			Label:      label,
		})
	}
	return resp
}

func GetPersonRelations(ctx *vbeam.Context, req GetPersonRelationsRequest) (resp GetPersonRelationsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopePeople, AccessView) {
		err = ErrPersonNotFound
		return
	}
	resp = personRelations(ctx.Tx, user, person)
	return
}

// GetRelationLabels names everyone the caller can see relative to one subject,
// so a page can group people by how they relate to the person it is about.
func GetRelationLabels(ctx *vbeam.Context, req GetRelationLabelsRequest) (resp GetRelationLabelsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	subject := GetPersonById(ctx.Tx, req.SubjectId)
	if !CanAccessPerson(ctx.Tx, user, subject, ScopePeople, AccessView) {
		err = ErrPersonNotFound
		return
	}

	resp.SubjectId = subject.Id
	resp.Labels = []RelationLabelEntry{}
	for _, person := range GetVisiblePeople(ctx.Tx, user) {
		match, found := relationBetween(ctx.Tx, subject, person)
		if !found {
			continue
		}
		resp.Labels = append(resp.Labels, RelationLabelEntry{
			PersonId: person.Id,
			Label:    match.term.forGender(person.Gender),
			Group:    match.group,
		})
	}
	return
}

func AddRelation(ctx *vbeam.Context, req AddRelationRequest) (resp RelationActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if !req.Stated.valid() || req.Stated == StatedNone {
		resp.Error = "Pick how these two are related"
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	anchor := GetPersonById(ctx.Tx, req.AnchorId)
	if person.Id == 0 || anchor.Id == 0 {
		err = ErrPersonNotFound
		return
	}
	if err = RequireFamilyAccess(ctx.Tx, user, person.FamilyId, AccessContribute); err != nil {
		return
	}
	if !CanAccessPerson(ctx.Tx, user, anchor, ScopePeople, AccessView) {
		err = ErrPersonNotFound
		return
	}

	anchorIds := append([]int{req.AnchorId}, req.AdditionalAnchorIds...)

	vbeam.UseWriteTx(ctx)
	if addErr := applyStatedRelationTx(ctx.Tx, user, person.Id, req.Stated, anchorIds); addErr != nil {
		if addErr == ErrPersonNotFound {
			err = addErr
			return
		}
		resp.Error = addErr.Error()
		return
	}
	relations := personRelations(ctx.Tx, user, person)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Relations = relations
	return
}

func RemoveRelation(ctx *vbeam.Context, req RemoveRelationRequest) (resp RelationActionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	var row Relation
	if !vbolt.Read(ctx.Tx, RelationBkt, req.RelationId, &row) {
		resp.Error = "That relationship no longer exists"
		return
	}

	person := GetPersonById(ctx.Tx, row.FromId)
	other := GetPersonById(ctx.Tx, row.ToId)
	fromErr := RequireFamilyAccess(ctx.Tx, user, person.FamilyId, AccessContribute)
	toErr := RequireFamilyAccess(ctx.Tx, user, other.FamilyId, AccessContribute)
	if fromErr != nil && toErr != nil {
		err = ErrFamilyAccessDenied
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteRelationTx(ctx.Tx, row)
	subject := person
	if fromErr != nil {
		subject = other
	}
	relations := personRelations(ctx.Tx, user, subject)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.Relations = relations
	return
}
