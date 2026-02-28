package backend

import (
	"errors"
	"family/cfg"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Tag struct
type Tag struct {
	Id        int       `json:"id"`
	FamilyId  int       `json:"familyId"`
	Name      string    `json:"name"`
	Color     string    `json:"color"` // hex string, e.g. "#4A90D9"
	CreatedAt time.Time `json:"createdAt"`
}

// Pack function for vbolt serialization
func PackTag(self *Tag, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Color, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// Buckets and indexes
var TagBkt = vbolt.Bucket(&cfg.Info, "tags", vpack.FInt, PackTag)
var TagByFamilyIndex = vbolt.Index(&cfg.Info, "tags_by_family", vpack.FInt, vpack.FInt)

// Request/Response types
type CreateTagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CreateTagResponse struct {
	Tag Tag `json:"tag"`
}

type UpdateTagRequest struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type UpdateTagResponse struct {
	Tag Tag `json:"tag"`
}

type DeleteTagRequest struct {
	Id int `json:"id"`
}

type DeleteTagResponse struct{}

type ListTagsRequest struct{}

type ListTagsResponse struct {
	Tags []Tag `json:"tags"`
}

// Tx helpers
func getTagById(tx *vbolt.Tx, id int) (tag Tag) {
	vbolt.Read(tx, TagBkt, id, &tag)
	return
}

func getTagsByFamily(tx *vbolt.Tx, familyId int) []Tag {
	var tagIds []int
	vbolt.ReadTermTargets(tx, TagByFamilyIndex, familyId, &tagIds, vbolt.Window{})
	if len(tagIds) == 0 {
		return []Tag{}
	}
	var tags []Tag
	vbolt.ReadSlice(tx, TagBkt, tagIds, &tags)
	return tags
}

func tagNameExistsInFamily(tx *vbolt.Tx, name string, familyId int, excludeId int) bool {
	tags := getTagsByFamily(tx, familyId)
	lowerName := strings.ToLower(name)
	for _, tag := range tags {
		if tag.Id == excludeId {
			continue
		}
		if strings.ToLower(tag.Name) == lowerName {
			return true
		}
	}
	return false
}

// RPC procedures
func CreateTag(ctx *vbeam.Context, req CreateTagRequest) (resp CreateTagResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		err = errors.New("Tag name is required")
		return
	}
	if len(name) > 40 {
		err = errors.New("Tag name must be 40 characters or fewer")
		return
	}

	vbeam.UseWriteTx(ctx)

	if tagNameExistsInFamily(ctx.Tx, name, user.FamilyId, -1) {
		err = errors.New("A tag with this name already exists")
		return
	}

	tag := Tag{
		Id:        vbolt.NextIntId(ctx.Tx, TagBkt),
		FamilyId:  user.FamilyId,
		Name:      name,
		Color:     req.Color,
		CreatedAt: time.Now(),
	}

	vbolt.Write(ctx.Tx, TagBkt, tag.Id, &tag)
	vbolt.SetTargetSingleTerm(ctx.Tx, TagByFamilyIndex, tag.Id, tag.FamilyId)
	vbolt.TxCommit(ctx.Tx)

	resp.Tag = tag
	return
}

func UpdateTag(ctx *vbeam.Context, req UpdateTagRequest) (resp UpdateTagResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Id <= 0 {
		err = errors.New("Tag ID is required")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		err = errors.New("Tag name is required")
		return
	}
	if len(name) > 40 {
		err = errors.New("Tag name must be 40 characters or fewer")
		return
	}

	vbeam.UseWriteTx(ctx)

	tag := getTagById(ctx.Tx, req.Id)
	if tag.Id == 0 {
		err = errors.New("Tag not found")
		return
	}
	if tag.FamilyId != user.FamilyId {
		err = errors.New("Access denied")
		return
	}

	if tagNameExistsInFamily(ctx.Tx, name, user.FamilyId, tag.Id) {
		err = errors.New("A tag with this name already exists")
		return
	}

	tag.Name = name
	tag.Color = req.Color

	vbolt.Write(ctx.Tx, TagBkt, tag.Id, &tag)
	vbolt.TxCommit(ctx.Tx)

	resp.Tag = tag
	return
}

func DeleteTag(ctx *vbeam.Context, req DeleteTagRequest) (resp DeleteTagResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	vbeam.UseWriteTx(ctx)

	tag := getTagById(ctx.Tx, req.Id)
	if tag.Id == 0 {
		err = errors.New("Tag not found")
		return
	}
	if tag.FamilyId != user.FamilyId {
		err = errors.New("Access denied")
		return
	}

	removeMilestoneTagsByTag(ctx.Tx, tag.Id)
	vbolt.SetTargetSingleTerm(ctx.Tx, TagByFamilyIndex, tag.Id, -1)
	vbolt.Delete(ctx.Tx, TagBkt, tag.Id)
	vbolt.TxCommit(ctx.Tx)
	return
}

func ListTags(ctx *vbeam.Context, req ListTagsRequest) (resp ListTagsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	tags := getTagsByFamily(ctx.Tx, user.FamilyId)
	sort.Slice(tags, func(i, j int) bool {
		return strings.ToLower(tags[i].Name) < strings.ToLower(tags[j].Name)
	})

	resp.Tags = tags
	return
}

func RegisterTagMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CreateTag)
	vbeam.RegisterProc(app, UpdateTag)
	vbeam.RegisterProc(app, DeleteTag)
	vbeam.RegisterProc(app, ListTags)
}
