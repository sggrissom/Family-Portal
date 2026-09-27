package backend

import (
	"errors"
	"math"
	"sort"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterSameAgeMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetSameAge)
}

const (
	sameAgePhotos          = 6
	sameAgePhotoCandidates = 24
	sameAgeMaxMonths       = 100 * 12
	daysPerMonth           = 30.4375
)

// sameAgeStep is how far one tap on the age control moves: a month for
// babies, a season for small children, half a year after that.
func sameAgeStep(ageMonths int) int {
	switch {
	case ageMonths < 24:
		return 1
	case ageMonths < 72:
		return 3
	default:
		return 6
	}
}

// Photos and milestones count as "at this age" within half a step either
// side; measurements are sparser, so the nearest within a whole step.
func sameAgeRecordWindow(ageMonths int) time.Duration {
	return monthsDuration(float64(sameAgeStep(ageMonths)) / 2)
}

func sameAgeGrowthWindow(ageMonths int) time.Duration {
	return monthsDuration(float64(sameAgeStep(ageMonths)))
}

func monthsDuration(months float64) time.Duration {
	return time.Duration(months * daysPerMonth * 24 * float64(time.Hour))
}

type GetSameAgeRequest struct {
	// AgeMonths null starts from the person's current age, or the youngest
	// child's when no person is given.
	AgeMonths    *int   `json:"ageMonths"`
	FromPersonId int    `json:"fromPersonId"`
	Today        string `json:"today"`
}

type SameAgeRow struct {
	Person     Person      `json:"person"`
	Date       time.Time   `json:"date"`
	Height     *GrowthData `json:"height"`
	Weight     *GrowthData `json:"weight"`
	Milestones []Milestone `json:"milestones"`
	PhotoIds   []int       `json:"photoIds"`
}

type GetSameAgeResponse struct {
	AgeMonths    int          `json:"ageMonths"`
	FromPersonId int          `json:"fromPersonId"`
	MaxAgeMonths int          `json:"maxAgeMonths"`
	Rows         []SameAgeRow `json:"rows"`
}

var ErrInvalidAge = errors.New("Age must be between 0 and 100 years")

func GetSameAge(ctx *vbeam.Context, req GetSameAgeRequest) (resp GetSameAgeResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if req.AgeMonths != nil && (*req.AgeMonths < 0 || *req.AgeMonths > sameAgeMaxMonths) {
		err = ErrInvalidAge
		return
	}

	today := dashboardToday(req.Today)
	timeline, err := GetFamilyTimeline(ctx, GetFamilyTimelineRequest{SkipPhotos: true})
	if err != nil {
		return
	}

	var candidates []FamilyTimelineItem
	for _, item := range timeline.People {
		if item.Person.IsPregnancy || item.Person.Birthday.IsZero() || item.Person.Birthday.After(today) {
			continue
		}
		candidates = append(candidates, item)
		resp.MaxAgeMonths = max(resp.MaxAgeMonths, monthsBetween(item.Person.Birthday, today))
	}

	resp.FromPersonId, resp.AgeMonths = sameAgeDefaults(ctx, user, candidates, req, today)
	resp.Rows = []SameAgeRow{}

	recordWindow := sameAgeRecordWindow(resp.AgeMonths)
	growthWindow := sameAgeGrowthWindow(resp.AgeMonths)

	for _, item := range candidates {
		target := item.Person.Birthday.AddDate(0, resp.AgeMonths, 0)
		if dayOf(target) > dayOf(today) {
			continue
		}
		row := SameAgeRow{Person: item.Person, Date: target, Milestones: []Milestone{}, PhotoIds: []int{}}

		for _, milestone := range item.Milestones {
			if absDuration(milestone.MilestoneDate.Sub(target)) <= recordWindow {
				row.Milestones = append(row.Milestones, milestone)
			}
		}
		sort.Slice(row.Milestones, func(i, j int) bool {
			return row.Milestones[i].MilestoneDate.Before(row.Milestones[j].MilestoneDate)
		})

		row.Height = nearestGrowth(item.GrowthData, Height, target, growthWindow)
		row.Weight = nearestGrowth(item.GrowthData, Weight, target, growthWindow)

		row.PhotoIds, err = nearestPhotoIds(ctx, item.Person.Id, target, recordWindow)
		if err != nil {
			return
		}
		resp.Rows = append(resp.Rows, row)
	}

	sortSameAgeRows(resp.Rows, resp.FromPersonId)
	return
}

func sameAgeDefaults(ctx *vbeam.Context, user User, candidates []FamilyTimelineItem, req GetSameAgeRequest, today time.Time) (fromPersonId int, ageMonths int) {
	var from *Person
	for i := range candidates {
		if candidates[i].Person.Id == req.FromPersonId {
			from = &candidates[i].Person
		}
	}
	if from == nil {
		for i := range candidates {
			person := &candidates[i].Person
			if !CanAccessFamily(ctx.Tx, user, person.FamilyId, AccessContribute) {
				continue
			}
			if from == nil || person.Birthday.After(from.Birthday) {
				from = person
			}
		}
	}
	if from != nil {
		fromPersonId = from.Id
	}

	if req.AgeMonths != nil {
		ageMonths = *req.AgeMonths
	} else if from != nil {
		ageMonths = max(0, monthsBetween(from.Birthday, today))
	}
	return
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func nearestGrowth(growth []GrowthData, kind MeasurementType, target time.Time, window time.Duration) *GrowthData {
	var best *GrowthData
	bestGap := time.Duration(math.MaxInt64)
	for i := range growth {
		if growth[i].MeasurementType != kind {
			continue
		}
		gap := absDuration(growth[i].MeasurementDate.Sub(target))
		if gap <= window && gap < bestGap {
			best = &growth[i]
			bestGap = gap
		}
	}
	return best
}

func nearestPhotoIds(ctx *vbeam.Context, personId int, target time.Time, window time.Duration) ([]int, error) {
	photos, err := ListFamilyPhotos(ctx, ListFamilyPhotosRequest{
		PersonId: personId,
		DateFrom: dayOf(target.Add(-window)),
		DateTo:   dayOf(target.Add(window)),
		Limit:    sameAgePhotoCandidates,
	})
	if err != nil {
		return nil, err
	}
	images := make([]Image, 0, len(photos.Photos))
	for _, photo := range photos.Photos {
		images = append(images, photo.Image)
	}
	sort.SliceStable(images, func(i, j int) bool {
		return absDuration(images[i].PhotoDate.Sub(target)) < absDuration(images[j].PhotoDate.Sub(target))
	})
	ids := []int{}
	for _, image := range images {
		if len(ids) == sameAgePhotos {
			break
		}
		ids = append(ids, image.Id)
	}
	return ids, nil
}

func (row SameAgeRow) hasRecords() bool {
	return row.Height != nil || row.Weight != nil || len(row.Milestones) > 0 || len(row.PhotoIds) > 0
}

// sortSameAgeRows puts the person the comparison starts from first, then
// everyone with something at that age, then the rest; youngest first within
// each.
func sortSameAgeRows(rows []SameAgeRow, fromPersonId int) {
	rank := func(row SameAgeRow) int {
		switch {
		case row.Person.Id == fromPersonId:
			return 0
		case row.hasRecords():
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if ri, rj := rank(rows[i]), rank(rows[j]); ri != rj {
			return ri < rj
		}
		return rows[i].Person.Birthday.After(rows[j].Person.Birthday)
	})
}
