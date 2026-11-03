package backend

import (
	"errors"
	"family/cfg"
	"sort"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Age input used to be stored as the day the person turned that age, so a
// family that recorded "at age 4" a dozen times has a dozen moments on the
// fourth birthday. The review finds those clusters; nothing changes until the
// family picks a correction.

const (
	RecordKindMoment = "moment"
	RecordKindPhoto  = "photo"
)

const birthdayClusterMinimum = 2

type BirthdayClusterRecord struct {
	Kind       string     `json:"kind"`
	Id         int        `json:"id"`
	Text       string     `json:"text"`
	PhotoId    int        `json:"photoId"`
	RecordTime RecordTime `json:"recordTime"`
}

type BirthdayCluster struct {
	PersonId int                     `json:"personId"`
	Age      int                     `json:"age"`
	Day      string                  `json:"day"`
	Records  []BirthdayClusterRecord `json:"records"`
}

type ListBirthdayClustersResponse struct {
	Clusters []BirthdayCluster `json:"clusters"`
	People   []Person          `json:"people"`
	Enabled  bool              `json:"enabled"`
}

type RecordTimeChange struct {
	Kind       string     `json:"kind"`
	Id         int        `json:"id"`
	RecordTime RecordTime `json:"recordTime"`
}

type CorrectRecordTimesRequest struct {
	Changes []RecordTimeChange `json:"changes"`
}

type CorrectRecordTimesResponse struct {
	CorrectionId int `json:"correctionId"`
	Changed      int `json:"changed"`
}

type UndoRecordTimeCorrectionRequest struct {
	CorrectionId int `json:"correctionId"`
}

type UndoRecordTimeCorrectionResponse struct {
	Restored int `json:"restored"`
	Skipped  int `json:"skipped"`
}

type recordTimeEdit struct {
	Kind   string
	Id     int
	Before RecordTime
	After  RecordTime
}

// RecordTimeCorrection is one applied bulk correction, kept so it can be
// undone.
type RecordTimeCorrection struct {
	Id        int
	UserId    int
	CreatedAt time.Time
	Edits     []recordTimeEdit
	Undone    bool
}

func packRecordTimeEdit(self *recordTimeEdit, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Kind, buf)
	vpack.Int(&self.Id, buf)
	PackRecordTime(&self.Before, buf)
	PackRecordTime(&self.After, buf)
}

func PackRecordTimeCorrection(self *RecordTimeCorrection, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.UserId, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Slice(&self.Edits, packRecordTimeEdit, buf)
	vpack.Bool(&self.Undone, buf)
}

var RecordTimeCorrectionBkt = vbolt.Bucket(&cfg.Info, "record_time_corrections", vpack.FInt, PackRecordTimeCorrection)

var ErrCorrectionNotFound = errors.New("That correction could not be found")

func RegisterRecordTimeReviewMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListBirthdayClusters)
	vbeam.RegisterProc(app, CorrectRecordTimes)
	vbeam.RegisterProc(app, UndoRecordTimeCorrection)
}

// birthdayAge reports which birthday a legacy exact day falls on, the way the
// old age input produced it: the birthday plus whole years via AddDate.
func birthdayAge(person Person, rt RecordTime) (int, bool) {
	if !rt.ExactDay() || !anchorOf(person).BirthdayKnown {
		return 0, false
	}
	born := dayStart(person.Birthday)
	day, _, _, _ := parseCivil(rt.From)
	age := day.Year - born.Year()
	if age < 1 || dayOf(born.AddDate(age, 0, 0)) != rt.From {
		return 0, false
	}
	return age, true
}

func ListBirthdayClusters(ctx *vbeam.Context, req Empty) (resp ListBirthdayClustersResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	resp.Enabled = ApproximateDatesEnabled()
	resp.Clusters = []BirthdayCluster{}
	resp.People = []Person{}

	for _, person := range GetVisiblePeople(ctx.Tx, user) {
		if !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
			continue
		}
		byAge := map[int][]BirthdayClusterRecord{}
		for _, m := range GetPersonMilestonesTx(ctx.Tx, person.Id) {
			if m.InterviewId > 0 || m.Category == CategoryInterview {
				continue
			}
			if age, ok := birthdayAge(person, m.RecordTime); ok {
				byAge[age] = append(byAge[age], BirthdayClusterRecord{
					Kind: RecordKindMoment, Id: m.Id, Text: m.Description,
					PhotoId: firstOf(GetMilestonePhotoIds(ctx.Tx, m.Id)), RecordTime: m.RecordTime,
				})
			}
		}
		for _, image := range GetPersonImages(ctx.Tx, person.Id) {
			if image.Status == 2 || !CanAccessFamily(ctx.Tx, user, image.FamilyId, AccessContribute) {
				continue
			}
			if age, ok := birthdayAge(person, image.RecordTime); ok {
				byAge[age] = append(byAge[age], BirthdayClusterRecord{
					Kind: RecordKindPhoto, Id: image.Id, Text: image.Title, PhotoId: image.Id, RecordTime: image.RecordTime,
				})
			}
		}
		added := false
		for age, records := range byAge {
			if len(records) < birthdayClusterMinimum {
				continue
			}
			sort.Slice(records, func(i, j int) bool {
				if records[i].Kind != records[j].Kind {
					return records[i].Kind < records[j].Kind
				}
				return records[i].Id < records[j].Id
			})
			resp.Clusters = append(resp.Clusters, BirthdayCluster{
				PersonId: person.Id, Age: age, Day: records[0].RecordTime.From, Records: records,
			})
			added = true
		}
		if added {
			resp.People = append(resp.People, person)
		}
	}
	sort.Slice(resp.Clusters, func(i, j int) bool {
		a, b := resp.Clusters[i], resp.Clusters[j]
		if len(a.Records) != len(b.Records) {
			return len(a.Records) > len(b.Records)
		}
		return a.Day < b.Day
	})
	return
}

func firstOf(ids []int) int {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

// setRecordTime applies one change after checking the user may edit the
// record, returning what was there before.
func setRecordTime(tx *vbolt.Tx, user User, kind string, id int, rt RecordTime) (before RecordTime, err error) {
	switch kind {
	case RecordKindMoment:
		m, err := GetMilestoneForUser(tx, id, user, AccessContribute)
		if err != nil {
			return before, err
		}
		if m.Category == CategoryInterview {
			return before, ErrInterviewCategory
		}
		before = m.RecordTime
		if err := applyRecordTime(tx, m.FamilyId, user.Id, rt, true, &m.RecordTime, &m.MilestoneDate); err != nil {
			return before, err
		}
		writeMilestone(tx, &m)
		return before, nil
	case RecordKindPhoto:
		image := GetImageById(tx, id)
		if image.Id == 0 || !CanAccessPhoto(tx, user, image, AccessContribute) {
			return before, errors.New("Photo not found or access denied")
		}
		before = image.RecordTime
		if err := applyRecordTime(tx, image.FamilyId, user.Id, rt, false, &image.RecordTime, &image.PhotoDate); err != nil {
			return before, err
		}
		writeImage(tx, &image)
		ReindexPhotoDates(tx, image.Id)
		return before, nil
	}
	return before, errors.New("Unknown record kind")
}

func currentRecordTime(tx *vbolt.Tx, kind string, id int) RecordTime {
	if kind == RecordKindMoment {
		return GetMilestoneById(tx, id).RecordTime
	}
	return GetImageById(tx, id).RecordTime
}

func CorrectRecordTimes(ctx *vbeam.Context, req CorrectRecordTimesRequest) (resp CorrectRecordTimesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if len(req.Changes) == 0 || len(req.Changes) > 500 {
		err = errors.New("Choose between 1 and 500 records to change")
		return
	}
	vbeam.UseWriteTx(ctx)

	correction := RecordTimeCorrection{UserId: user.Id, CreatedAt: time.Now().UTC()}
	for _, change := range req.Changes {
		before, applyErr := setRecordTime(ctx.Tx, user, change.Kind, change.Id, change.RecordTime)
		if applyErr != nil {
			err = applyErr
			return
		}
		after := currentRecordTime(ctx.Tx, change.Kind, change.Id)
		correction.Edits = append(correction.Edits, recordTimeEdit{Kind: change.Kind, Id: change.Id, Before: before, After: after})
	}
	correction.Id = vbolt.NextIntId(ctx.Tx, RecordTimeCorrectionBkt)
	vbolt.Write(ctx.Tx, RecordTimeCorrectionBkt, correction.Id, &correction)

	resp = CorrectRecordTimesResponse{CorrectionId: correction.Id, Changed: len(correction.Edits)}
	vbolt.TxCommit(ctx.Tx)
	return
}

// UndoRecordTimeCorrection restores each record the correction changed,
// unless someone has changed its date again since.
func UndoRecordTimeCorrection(ctx *vbeam.Context, req UndoRecordTimeCorrectionRequest) (resp UndoRecordTimeCorrectionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)

	var correction RecordTimeCorrection
	vbolt.Read(ctx.Tx, RecordTimeCorrectionBkt, req.CorrectionId, &correction)
	if correction.Id == 0 || correction.UserId != user.Id || correction.Undone {
		err = ErrCorrectionNotFound
		return
	}
	for _, edit := range correction.Edits {
		if currentRecordTime(ctx.Tx, edit.Kind, edit.Id) != edit.After {
			resp.Skipped++
			continue
		}
		if restoreErr := restoreRecordTime(ctx.Tx, user, edit); restoreErr != nil {
			resp.Skipped++
			continue
		}
		resp.Restored++
	}
	correction.Undone = true
	vbolt.Write(ctx.Tx, RecordTimeCorrectionBkt, correction.Id, &correction)
	vbolt.TxCommit(ctx.Tx)
	return
}

func restoreRecordTime(tx *vbolt.Tx, user User, edit recordTimeEdit) error {
	restore := func(rt *RecordTime, at *time.Time) {
		*rt = edit.Before
		if rt.ExactDay() {
			start, _, _, _ := parseCivil(rt.From)
			if civilOf(*at).String() != rt.From {
				*at = start.Time()
			}
		}
	}
	switch edit.Kind {
	case RecordKindMoment:
		m, err := GetMilestoneForUser(tx, edit.Id, user, AccessContribute)
		if err != nil {
			return err
		}
		restore(&m.RecordTime, &m.MilestoneDate)
		writeMilestone(tx, &m)
	case RecordKindPhoto:
		image := GetImageById(tx, edit.Id)
		if image.Id == 0 || !CanAccessPhoto(tx, user, image, AccessContribute) {
			return errors.New("Photo not found or access denied")
		}
		restore(&image.RecordTime, &image.PhotoDate)
		writeImage(tx, &image)
		ReindexPhotoDates(tx, image.Id)
	}
	return nil
}
