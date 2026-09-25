package backend

import (
	"errors"
	"family/cfg"
	"math"
	"sort"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

const (
	FaceUnknown   = 0
	FaceAuto      = 1
	FaceConfirmed = 2
	FaceDismissed = 3
)

const (
	faceMatchThreshold   = 0.5
	faceSuggestThreshold = 0.62
	faceNearestRefs      = 3
	faceAmbiguityMargin  = 0.04
	faceClusterThreshold = 0.45
	faceClusterCompare   = 12
	faceRefsPerPerson    = 100
	faceReviewAutoLimit  = 60
)

type FaceBox struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

type PhotoFace struct {
	Id                int       `json:"id"`
	PhotoId           int       `json:"photoId"`
	FamilyId          int       `json:"familyId"`
	PersonId          int       `json:"personId"`
	Status            int       `json:"status"`
	Distance          float64   `json:"distance"`
	Box               FaceBox   `json:"box"`
	RejectedPersonIds []int     `json:"-"`
	Descriptor        []float32 `json:"-"`
	CreatedAt         time.Time `json:"createdAt"`
}

func PackFaceBox(self *FaceBox, buf *vpack.Buffer) {
	vpack.Float64(&self.Left, buf)
	vpack.Float64(&self.Top, buf)
	vpack.Float64(&self.Right, buf)
	vpack.Float64(&self.Bottom, buf)
}

func PackPhotoFace(self *PhotoFace, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.Status, buf)
	vpack.Float64(&self.Distance, buf)
	PackFaceBox(&self.Box, buf)
	vpack.Slice(&self.RejectedPersonIds, vpack.Int, buf)
	packFloat32Slice(&self.Descriptor, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var PhotoFaceBkt = vbolt.Bucket(&cfg.Info, "photo_faces", vpack.FInt, PackPhotoFace)
var FaceByPhotoIndex = vbolt.Index(&cfg.Info, "face_by_photo", vpack.FInt, vpack.FInt)
var FaceByFamilyIndex = vbolt.Index(&cfg.Info, "face_by_family", vpack.FInt, vpack.FInt)
var FaceByPersonIndex = vbolt.Index(&cfg.Info, "face_by_person", vpack.FInt, vpack.FInt)

type DetectedFace struct {
	Descriptor []float32
	Box        FaceBox
}

func readFaces(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], term int) (faces []PhotoFace) {
	var ids []int
	vbolt.ReadTermTargets(tx, index, term, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoFaceBkt, ids, &faces)
	return
}

func GetPhotoFacesTx(tx *vbolt.Tx, photoId int) []PhotoFace {
	return readFaces(tx, FaceByPhotoIndex, photoId)
}

func GetFamilyFaces(tx *vbolt.Tx, familyId int) []PhotoFace {
	return readFaces(tx, FaceByFamilyIndex, familyId)
}

func GetPersonFaces(tx *vbolt.Tx, personId int) []PhotoFace {
	return readFaces(tx, FaceByPersonIndex, personId)
}

func writeFace(tx *vbolt.Tx, face *PhotoFace) {
	vbolt.Write(tx, PhotoFaceBkt, face.Id, face)
	vbolt.SetTargetSingleTerm(tx, FaceByPhotoIndex, face.Id, face.PhotoId)
	vbolt.SetTargetSingleTerm(tx, FaceByFamilyIndex, face.Id, face.FamilyId)
	if face.PersonId > 0 {
		vbolt.SetTargetSingleTerm(tx, FaceByPersonIndex, face.Id, face.PersonId)
	} else {
		vbolt.DeleteTargetTerms(tx, FaceByPersonIndex, face.Id)
	}
}

func deleteFace(tx *vbolt.Tx, faceId int) {
	vbolt.Delete(tx, PhotoFaceBkt, faceId)
	vbolt.DeleteTargetTerms(tx, FaceByPhotoIndex, faceId)
	vbolt.DeleteTargetTerms(tx, FaceByFamilyIndex, faceId)
	vbolt.DeleteTargetTerms(tx, FaceByPersonIndex, faceId)
}

func deletePhotoFacesTx(tx *vbolt.Tx, photoId int) {
	for _, face := range GetPhotoFacesTx(tx, photoId) {
		deleteFace(tx, face.Id)
	}
}

func faceEuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	var sum float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

func boxIoU(a, b FaceBox) float64 {
	left := math.Max(a.Left, b.Left)
	top := math.Max(a.Top, b.Top)
	right := math.Min(a.Right, b.Right)
	bottom := math.Min(a.Bottom, b.Bottom)
	if right <= left || bottom <= top {
		return 0
	}
	inter := (right - left) * (bottom - top)
	area := func(box FaceBox) float64 {
		return (box.Right - box.Left) * (box.Bottom - box.Top)
	}
	return inter / (area(a) + area(b) - inter)
}

type faceReference struct {
	PersonId    int
	Descriptors [][]float32
}

func familyFaceReferences(tx *vbolt.Tx, familyId int) []faceReference {
	var refs []faceReference
	for _, person := range GetFamilyOwnPeople(tx, familyId) {
		ref := faceReference{PersonId: person.Id}
		if len(person.FaceDescriptor) == 128 {
			ref.Descriptors = append(ref.Descriptors, person.FaceDescriptor)
		}
		faces := GetPersonFaces(tx, person.Id)
		sort.Slice(faces, func(i, j int) bool { return faces[i].Id > faces[j].Id })
		for _, face := range faces {
			if len(ref.Descriptors) >= faceRefsPerPerson {
				break
			}
			if face.Status == FaceConfirmed && len(face.Descriptor) == 128 {
				ref.Descriptors = append(ref.Descriptors, face.Descriptor)
			}
		}
		if len(ref.Descriptors) > 0 {
			refs = append(refs, ref)
		}
	}
	return refs
}

type faceCandidate struct {
	PersonId int
	Distance float64
}

func rankFaceCandidates(descriptor []float32, refs []faceReference, rejected []int) []faceCandidate {
	var out []faceCandidate
	for _, ref := range refs {
		if containsInt(rejected, ref.PersonId) {
			continue
		}
		out = append(out, faceCandidate{PersonId: ref.PersonId, Distance: referenceDistance(descriptor, ref)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Distance < out[j].Distance })
	return out
}

// Averages the closest few references, so one lucky lookalike among many
// confirmed faces is not enough to claim a match.
func referenceDistance(descriptor []float32, ref faceReference) float64 {
	dists := make([]float64, 0, len(ref.Descriptors))
	for _, d := range ref.Descriptors {
		dists = append(dists, faceEuclideanDistance(descriptor, d))
	}
	sort.Float64s(dists)
	n := min(len(dists), faceNearestRefs)
	sum := 0.0
	for _, d := range dists[:n] {
		sum += d
	}
	return sum / float64(n)
}

func confidentMatch(ranked []faceCandidate) (faceCandidate, bool) {
	if len(ranked) == 0 || ranked[0].Distance >= faceMatchThreshold {
		return faceCandidate{}, false
	}
	if len(ranked) > 1 && ranked[1].Distance-ranked[0].Distance < faceAmbiguityMargin {
		return faceCandidate{}, false
	}
	return ranked[0], true
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func ensurePhotoPersonTx(tx *vbolt.Tx, photo Image, personId int, auto bool) {
	for _, pp := range GetPhotoPersonsByPhoto(tx, photo.Id) {
		if pp.PersonId == personId {
			if pp.AutoTagged && !auto {
				pp.AutoTagged = false
				vbolt.Write(tx, PhotoPersonBkt, pp.Id, &pp)
			}
			return
		}
	}
	id := AddPersonToPhoto(tx, photo.Id, personId, photo.FamilyId)
	if auto {
		pp := GetPhotoPersonById(tx, id)
		pp.AutoTagged = true
		vbolt.Write(tx, PhotoPersonBkt, pp.Id, &pp)
	}
}

func dropAutoTagIfUnbackedTx(tx *vbolt.Tx, photoId int, personId int) {
	for _, face := range GetPhotoFacesTx(tx, photoId) {
		if face.PersonId == personId {
			return
		}
	}
	for _, pp := range GetPhotoPersonsByPhoto(tx, photoId) {
		if pp.PersonId == personId && pp.AutoTagged {
			RemovePersonFromPhoto(tx, photoId, personId)
			return
		}
	}
}

// Assigns unknown faces within one photo to people, closest pairs first, so two
// faces never land on the same person and a person already placed on another
// face of the photo is skipped.
func autoAssignPhotoFacesTx(tx *vbolt.Tx, photo Image, faces []PhotoFace, refs []faceReference) (tagged int) {
	taken := make(map[int]bool)
	for _, face := range faces {
		if face.PersonId > 0 {
			taken[face.PersonId] = true
		}
	}

	type pick struct {
		idx int
		faceCandidate
	}
	var picks []pick
	for i, face := range faces {
		if face.Status != FaceUnknown || len(face.Descriptor) != 128 {
			continue
		}
		if match, ok := confidentMatch(rankFaceCandidates(face.Descriptor, refs, face.RejectedPersonIds)); ok {
			picks = append(picks, pick{i, match})
		}
	}
	sort.Slice(picks, func(i, j int) bool { return picks[i].Distance < picks[j].Distance })

	for _, p := range picks {
		if taken[p.PersonId] {
			continue
		}
		taken[p.PersonId] = true
		face := &faces[p.idx]
		face.PersonId = p.PersonId
		face.Status = FaceAuto
		face.Distance = p.Distance
		writeFace(tx, face)
		ensurePhotoPersonTx(tx, photo, p.PersonId, true)
		tagged++
	}
	return
}

// Replaces a photo's detected faces with a fresh detection, carrying forward
// any decisions already made about a face found in the same place.
func RecordPhotoFacesTx(tx *vbolt.Tx, photoId int, detected []DetectedFace) (tagged int) {
	photo := GetImageById(tx, photoId)
	if photo.Id == 0 {
		return
	}

	previous := GetPhotoFacesTx(tx, photoId)
	used := make([]bool, len(previous))
	faces := make([]PhotoFace, 0, len(detected))
	for _, d := range detected {
		face := PhotoFace{
			Id:         vbolt.NextIntId(tx, PhotoFaceBkt),
			PhotoId:    photo.Id,
			FamilyId:   photo.FamilyId,
			Box:        d.Box,
			Descriptor: d.Descriptor,
			CreatedAt:  time.Now(),
		}
		for i, old := range previous {
			if used[i] {
				continue
			}
			samePlace := boxIoU(old.Box, d.Box) > 0.5
			sameFace := faceEuclideanDistance(old.Descriptor, d.Descriptor) < 0.15
			if samePlace || sameFace {
				used[i] = true
				face.PersonId = old.PersonId
				face.Status = old.Status
				face.Distance = old.Distance
				face.RejectedPersonIds = old.RejectedPersonIds
				face.CreatedAt = old.CreatedAt
				break
			}
		}
		faces = append(faces, face)
	}

	for _, old := range previous {
		deleteFace(tx, old.Id)
	}
	for i := range faces {
		writeFace(tx, &faces[i])
	}

	return autoAssignPhotoFacesTx(tx, photo, faces, familyFaceReferences(tx, photo.FamilyId))
}

func RematchFamilyFacesTx(tx *vbolt.Tx, familyId int) (tagged int) {
	refs := familyFaceReferences(tx, familyId)
	if len(refs) == 0 {
		return
	}
	byPhoto := make(map[int][]PhotoFace)
	hasUnknown := make(map[int]bool)
	for _, face := range GetFamilyFaces(tx, familyId) {
		byPhoto[face.PhotoId] = append(byPhoto[face.PhotoId], face)
		if face.Status == FaceUnknown {
			hasUnknown[face.PhotoId] = true
		}
	}
	for photoId := range hasUnknown {
		photo := GetImageById(tx, photoId)
		if photo.Id == 0 {
			continue
		}
		tagged += autoAssignPhotoFacesTx(tx, photo, byPhoto[photoId], refs)
	}
	return
}

func unassignPersonFacesOnPhotoTx(tx *vbolt.Tx, photoId int, personId int, reject bool) {
	for _, face := range GetPhotoFacesTx(tx, photoId) {
		if face.PersonId != personId {
			continue
		}
		face.PersonId = 0
		face.Status = FaceUnknown
		face.Distance = 0
		if reject && !containsInt(face.RejectedPersonIds, personId) {
			face.RejectedPersonIds = append(face.RejectedPersonIds, personId)
		}
		writeFace(tx, &face)
	}
}

func unassignPersonFacesTx(tx *vbolt.Tx, personId int) {
	for _, face := range GetPersonFaces(tx, personId) {
		face.PersonId = 0
		face.Status = FaceUnknown
		face.Distance = 0
		writeFace(tx, &face)
	}
}

func moveFacesToPersonTx(tx *vbolt.Tx, fromPersonId int, toPersonId int) {
	for _, face := range GetPersonFaces(tx, fromPersonId) {
		face.PersonId = toPersonId
		writeFace(tx, &face)
	}
}

type faceCluster struct {
	faces  []PhotoFace
	photos map[int]bool
}

// A face joins a group only when it is close to every face already in it
// (bounded to the first few), so lookalike siblings are not chained together.
func clusterFaces(faces []PhotoFace) []faceCluster {
	var clusters []faceCluster
	for _, face := range faces {
		if len(face.Descriptor) != 128 {
			clusters = append(clusters, faceCluster{faces: []PhotoFace{face}})
			continue
		}
		best, bestDist := -1, math.MaxFloat64
		for i, c := range clusters {
			if len(c.faces[0].Descriptor) != 128 {
				continue
			}
			if c.photos[face.PhotoId] {
				continue
			}
			worst := 0.0
			for j, member := range c.faces {
				if j >= faceClusterCompare {
					break
				}
				worst = math.Max(worst, faceEuclideanDistance(face.Descriptor, member.Descriptor))
				if worst >= faceClusterThreshold {
					break
				}
			}
			if worst < faceClusterThreshold && worst < bestDist {
				best, bestDist = i, worst
			}
		}
		if best >= 0 {
			clusters[best].faces = append(clusters[best].faces, face)
			clusters[best].photos[face.PhotoId] = true
		} else {
			clusters = append(clusters, faceCluster{
				faces:  []PhotoFace{face},
				photos: map[int]bool{face.PhotoId: true},
			})
		}
	}
	sort.SliceStable(clusters, func(i, j int) bool { return len(clusters[i].faces) > len(clusters[j].faces) })
	return clusters
}

func clusterCentroid(c faceCluster) []float32 {
	var centroid []float32
	n := 0
	for _, face := range c.faces {
		if len(face.Descriptor) != 128 {
			continue
		}
		if centroid == nil {
			centroid = make([]float32, 128)
		}
		for k, v := range face.Descriptor {
			centroid[k] += v
		}
		n++
	}
	for k := range centroid {
		centroid[k] /= float32(n)
	}
	return centroid
}

func RegisterFaceMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetFaceReview)
	vbeam.RegisterProc(app, GetPhotoFaces)
	vbeam.RegisterProc(app, AssignFaces)
	vbeam.RegisterProc(app, RejectFaces)
	vbeam.RegisterProc(app, DismissFaces)
}

type FaceGroup struct {
	FamilyId           int         `json:"familyId"`
	Faces              []PhotoFace `json:"faces"`
	SuggestedPersonId  int         `json:"suggestedPersonId"`
	SuggestionDistance float64     `json:"suggestionDistance"`
}

type FaceReviewFamily struct {
	FamilyId int      `json:"familyId"`
	Name     string   `json:"name"`
	People   []Person `json:"people"`
}

type GetFaceReviewRequest struct{}

type GetFaceReviewResponse struct {
	Enabled      bool               `json:"enabled"`
	Groups       []FaceGroup        `json:"groups"`
	AutoTagged   []PhotoFace        `json:"autoTagged"`
	Families     []FaceReviewFamily `json:"families"`
	UnknownCount int                `json:"unknownCount"`
	AutoCount    int                `json:"autoCount"`
}

func GetFaceReview(ctx *vbeam.Context, req GetFaceReviewRequest) (resp GetFaceReviewResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	resp.Enabled = cfg.EnableFaceTagging
	resp.Groups = []FaceGroup{}
	resp.AutoTagged = []PhotoFace{}
	resp.Families = []FaceReviewFamily{}

	for _, familyId := range familiesVisibleTo(ctx.Tx, user) {
		if !CanAccessFamily(ctx.Tx, user, familyId, AccessContribute) {
			continue
		}
		people := GetFamilyOwnPeople(ctx.Tx, familyId)
		sort.Slice(people, func(i, j int) bool { return people[i].Name < people[j].Name })
		if people == nil {
			people = []Person{}
		}
		resp.Families = append(resp.Families, FaceReviewFamily{
			FamilyId: familyId,
			Name:     GetFamily(ctx.Tx, familyId).Name,
			People:   people,
		})

		var unknown, auto []PhotoFace
		for _, face := range GetFamilyFaces(ctx.Tx, familyId) {
			switch face.Status {
			case FaceUnknown:
				unknown = append(unknown, face)
			case FaceAuto:
				auto = append(auto, face)
			}
		}
		resp.UnknownCount += len(unknown)
		resp.AutoCount += len(auto)

		refs := familyFaceReferences(ctx.Tx, familyId)
		for _, c := range clusterFaces(unknown) {
			group := FaceGroup{FamilyId: familyId, Faces: c.faces}
			if centroid := clusterCentroid(c); centroid != nil {
				var rejected []int
				for _, face := range c.faces {
					rejected = append(rejected, face.RejectedPersonIds...)
				}
				ranked := rankFaceCandidates(centroid, refs, rejected)
				if len(ranked) > 0 && ranked[0].Distance < faceSuggestThreshold {
					group.SuggestedPersonId = ranked[0].PersonId
					group.SuggestionDistance = ranked[0].Distance
				}
			}
			resp.Groups = append(resp.Groups, group)
		}
		resp.AutoTagged = append(resp.AutoTagged, auto...)
	}

	sort.SliceStable(resp.AutoTagged, func(i, j int) bool {
		return resp.AutoTagged[i].Distance > resp.AutoTagged[j].Distance
	})
	if len(resp.AutoTagged) > faceReviewAutoLimit {
		resp.AutoTagged = resp.AutoTagged[:faceReviewAutoLimit]
	}
	return
}

type GetPhotoFacesRequest struct {
	PhotoId int `json:"photoId"`
}

type GetPhotoFacesResponse struct {
	Faces    []PhotoFace `json:"faces"`
	People   []Person    `json:"people"`
	CanLabel bool        `json:"canLabel"`
}

func GetPhotoFaces(ctx *vbeam.Context, req GetPhotoFacesRequest) (resp GetPhotoFacesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	resp.Faces = []PhotoFace{}
	resp.People = []Person{}

	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
		return
	}

	for _, face := range GetPhotoFacesTx(ctx.Tx, photo.Id) {
		if face.Status != FaceDismissed {
			resp.Faces = append(resp.Faces, face)
		}
	}
	sort.Slice(resp.Faces, func(i, j int) bool { return resp.Faces[i].Box.Left < resp.Faces[j].Box.Left })

	people := GetFamilyOwnPeople(ctx.Tx, photo.FamilyId)
	sort.Slice(people, func(i, j int) bool { return people[i].Name < people[j].Name })
	if people != nil {
		resp.People = people
	}
	resp.CanLabel = true
	return
}

var ErrFaceNotFound = errors.New("Face not found or access denied")

func loadFacesForEdit(ctx *vbeam.Context, faceIds []int) (user User, faces []PhotoFace, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if len(faceIds) == 0 {
		err = errors.New("At least one face is required")
		return
	}
	seen := make(map[int]bool)
	for _, id := range faceIds {
		if seen[id] {
			continue
		}
		seen[id] = true
		var face PhotoFace
		if !vbolt.Read(ctx.Tx, PhotoFaceBkt, id, &face) || !CanAccessFamily(ctx.Tx, user, face.FamilyId, AccessContribute) {
			err = ErrFaceNotFound
			return
		}
		faces = append(faces, face)
	}
	return
}

type AssignFacesRequest struct {
	FaceIds  []int `json:"faceIds"`
	PersonId int   `json:"personId"`
}

type AssignFacesResponse struct {
	Assigned   int `json:"assigned"`
	AutoTagged int `json:"autoTagged"`
}

func AssignFaces(ctx *vbeam.Context, req AssignFacesRequest) (resp AssignFacesResponse, err error) {
	_, faces, err := loadFacesForEdit(ctx, req.FaceIds)
	if err != nil {
		return
	}

	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 {
		err = errors.New("Person not found or access denied")
		return
	}
	for _, face := range faces {
		if !CanFamilyAccess(ctx.Tx, face.FamilyId, person.FamilyId, AccessContribute) {
			err = errors.New("Person not found or access denied")
			return
		}
	}

	vbeam.UseWriteTx(ctx)

	families := make(map[int]bool)
	for _, face := range faces {
		photo := GetImageById(ctx.Tx, face.PhotoId)
		if photo.Id == 0 {
			continue
		}
		previous := face.PersonId
		for _, other := range GetPhotoFacesTx(ctx.Tx, photo.Id) {
			if other.Id != face.Id && other.PersonId == person.Id {
				other.PersonId = 0
				other.Status = FaceUnknown
				other.Distance = 0
				if !containsInt(other.RejectedPersonIds, person.Id) {
					other.RejectedPersonIds = append(other.RejectedPersonIds, person.Id)
				}
				writeFace(ctx.Tx, &other)
			}
		}
		face.PersonId = person.Id
		face.Status = FaceConfirmed
		face.Distance = 0
		writeFace(ctx.Tx, &face)
		ensurePhotoPersonTx(ctx.Tx, photo, person.Id, false)
		if previous > 0 && previous != person.Id {
			dropAutoTagIfUnbackedTx(ctx.Tx, photo.Id, previous)
		}
		families[face.FamilyId] = true
		resp.Assigned++
	}

	for familyId := range families {
		resp.AutoTagged += RematchFamilyFacesTx(ctx.Tx, familyId)
	}

	vbolt.TxCommit(ctx.Tx)
	return
}

type FaceIdsRequest struct {
	FaceIds []int `json:"faceIds"`
}

type FaceIdsResponse struct {
	Updated int `json:"updated"`
}

func RejectFaces(ctx *vbeam.Context, req FaceIdsRequest) (resp FaceIdsResponse, err error) {
	_, faces, err := loadFacesForEdit(ctx, req.FaceIds)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	for _, face := range faces {
		if face.PersonId == 0 {
			continue
		}
		personId := face.PersonId
		wasAuto := face.Status == FaceAuto
		face.PersonId = 0
		face.Status = FaceUnknown
		face.Distance = 0
		if !containsInt(face.RejectedPersonIds, personId) {
			face.RejectedPersonIds = append(face.RejectedPersonIds, personId)
		}
		writeFace(ctx.Tx, &face)
		if wasAuto {
			dropAutoTagIfUnbackedTx(ctx.Tx, face.PhotoId, personId)
		}
		resp.Updated++
	}
	vbolt.TxCommit(ctx.Tx)
	return
}

func DismissFaces(ctx *vbeam.Context, req FaceIdsRequest) (resp FaceIdsResponse, err error) {
	_, faces, err := loadFacesForEdit(ctx, req.FaceIds)
	if err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)
	for _, face := range faces {
		personId := face.PersonId
		wasAuto := face.Status == FaceAuto
		face.PersonId = 0
		face.Status = FaceDismissed
		face.Distance = 0
		writeFace(ctx.Tx, &face)
		if wasAuto && personId > 0 {
			dropAutoTagIfUnbackedTx(ctx.Tx, face.PhotoId, personId)
		}
		resp.Updated++
	}
	vbolt.TxCommit(ctx.Tx)
	return
}
