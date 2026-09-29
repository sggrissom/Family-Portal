package backend

import (
	"errors"
	"math"
	"sort"
	"time"

	"go.hasen.dev/vbeam"
)

const (
	oftenWithLimit      = 8
	headerPhotoLookback = 18 * 30 * 24 * time.Hour
)

type GetPersonPhotoInsightsRequest struct {
	PersonId int `json:"personId"`
}

// PortraitPhoto is a photo chosen to show a person, with their face box when
// face analysis found it.
type PortraitPhoto struct {
	PhotoId int       `json:"photoId"`
	Box     FaceBox   `json:"box"`
	Date    time.Time `json:"date"`
	// AgeMonths is where this photo sits on the person's timeline: the month
	// under two, the year (in months) after, or -1 without a birthday.
	AgeMonths int `json:"ageMonths"`
	Year      int `json:"year"`
}

type OftenWith struct {
	Person   Person    `json:"person"`
	Count    int       `json:"count"`
	LastDate time.Time `json:"lastDate"`
}

type GetPersonPhotoInsightsResponse struct {
	GrowingUp []PortraitPhoto `json:"growingUp"`
	OftenWith []OftenWith     `json:"oftenWith"`
	Header    *PortraitPhoto  `json:"header"`
}

type candidatePortrait struct {
	PortraitPhoto
	score   float64
	hasFace bool
}

// portraitScore prefers a large, confirmed face in a sharp photo with few
// other people in it.
func portraitScore(face *PhotoFace, quality float64, peopleInPhoto int) float64 {
	score := 0.35*quality + 0.2/float64(max(peopleInPhoto, 1))
	if face != nil {
		area := (face.Box.Right - face.Box.Left) * (face.Box.Bottom - face.Box.Top)
		score += 0.45 * math.Min(1, math.Sqrt(math.Max(area, 0))/0.35)
		if face.Status == FaceConfirmed {
			score += 0.1
		}
	}
	return score
}

func timelineBucket(birthday, date time.Time) int {
	if birthday.IsZero() {
		return -1
	}
	months := ageInMonths(birthday, date)
	switch {
	case months < 0:
		return -1
	case months < 24:
		return months
	default:
		return months - months%12
	}
}

func GetPersonPhotoInsights(ctx *vbeam.Context, req GetPersonPhotoInsightsRequest) (resp GetPersonPhotoInsightsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	person := GetPersonById(ctx.Tx, req.PersonId)
	if !CanAccessPerson(ctx.Tx, user, person, ScopePhotos, AccessView) {
		err = errors.New("Person not found or not in your family")
		return
	}
	resp.GrowingUp = []PortraitPhoto{}
	resp.OftenWith = []OftenWith{}

	visible := map[int]Person{}
	for _, p := range GetVisiblePeople(ctx.Tx, user) {
		visible[p.Id] = p
	}
	withCount := map[int]*OftenWith{}
	best := map[int]candidatePortrait{}
	var header *candidatePortrait
	now := time.Now()

	stream := newPhotoStream(ImageByPersonDateIndex, person.Id, photoKey{seconds: math.MaxInt64, id: math.MaxInt})
	for stream.peek(ctx.Tx) {
		img := stream.pop()
		if img.Status != 0 || !CanAccessPhoto(ctx.Tx, user, img, AccessView) {
			continue
		}
		inPhoto := GetPhotoPersonsByPhoto(ctx.Tx, img.Id)
		for _, pp := range inPhoto {
			other, ok := visible[pp.PersonId]
			if pp.PersonId == person.Id || !ok {
				continue
			}
			w := withCount[other.Id]
			if w == nil {
				w = &OftenWith{Person: other}
				withCount[other.Id] = w
			}
			w.Count++
			if img.PhotoDate.After(w.LastDate) {
				w.LastDate = img.PhotoDate
			}
		}

		var face *PhotoFace
		for _, f := range GetPhotoFacesTx(ctx.Tx, img.Id) {
			if f.PersonId == person.Id && (f.Status == FaceAuto || f.Status == FaceConfirmed) {
				f := f
				face = &f
				break
			}
		}
		features, _ := GetPhotoFeatures(ctx.Tx, img.Id)
		c := candidatePortrait{
			PortraitPhoto: PortraitPhoto{
				PhotoId: img.Id, Date: img.PhotoDate, Year: img.PhotoDate.Year(),
				AgeMonths: timelineBucket(person.Birthday, img.PhotoDate),
			},
			score:   portraitScore(face, features.Quality, len(inPhoto)),
			hasFace: face != nil,
		}
		if face != nil {
			c.Box = face.Box
		}

		key := c.AgeMonths
		if key < 0 {
			key = -c.Year
		}
		if current, ok := best[key]; !ok || c.score > current.score {
			best[key] = c
		}
		if c.hasFace && now.Sub(img.PhotoDate) <= headerPhotoLookback && (header == nil || c.score > header.score) {
			copied := c
			header = &copied
		}
	}

	for _, c := range best {
		resp.GrowingUp = append(resp.GrowingUp, c.PortraitPhoto)
	}
	sort.Slice(resp.GrowingUp, func(i, j int) bool { return resp.GrowingUp[i].Date.Before(resp.GrowingUp[j].Date) })
	if header != nil {
		resp.Header = &header.PortraitPhoto
	}

	for _, w := range withCount {
		w.Person.Age = calculateAge(w.Person.Birthday)
		resp.OftenWith = append(resp.OftenWith, *w)
	}
	sort.Slice(resp.OftenWith, func(i, j int) bool {
		a, b := resp.OftenWith[i], resp.OftenWith[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.LastDate.After(b.LastDate)
	})
	resp.OftenWith = resp.OftenWith[:min(len(resp.OftenWith), oftenWithLimit)]
	return
}

func RegisterPersonPhotoInsightMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetPersonPhotoInsights)
}
