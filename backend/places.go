package backend

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"family/cfg"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

//go:embed geodata/cities.tsv.gz
var citiesData []byte

const (
	earthRadiusMeters    = 6371000.0
	maxCityDistance      = 75000.0
	defaultPlaceRadius   = 250
	maxFamilyPlaceRadius = 50000
	maxPlaceNameLength   = 80
)

type City struct {
	Id      int
	Name    string
	Region  string
	Country string
	Lat     float64
	Lng     float64
}

func (c City) Label() string {
	if c.Region != "" && c.Region != c.Name {
		return c.Name + ", " + c.Region
	}
	if c.Country != "" {
		return c.Name + ", " + c.Country
	}
	return c.Name
}

type cellKey struct{ lat, lng int }

type geocoder struct {
	cities []City
	byId   map[int]int
	cells  map[cellKey][]int
}

var (
	geocoderOnce sync.Once
	sharedGeo    *geocoder
)

func loadGeocoder() *geocoder {
	geocoderOnce.Do(func() {
		g := &geocoder{byId: map[int]int{}, cells: map[cellKey][]int{}}
		zr, err := gzip.NewReader(bytes.NewReader(citiesData))
		if err != nil {
			LogErrorSimple(LogCategoryWorker, "Failed to open embedded city data", map[string]interface{}{"error": err.Error()})
			sharedGeo = g
			return
		}
		scanner := bufio.NewScanner(zr)
		for scanner.Scan() {
			cols := strings.Split(scanner.Text(), "\t")
			if len(cols) != 6 {
				continue
			}
			id, _ := strconv.Atoi(cols[0])
			lat, _ := strconv.ParseFloat(cols[4], 64)
			lng, _ := strconv.ParseFloat(cols[5], 64)
			g.byId[id] = len(g.cities)
			key := cellOf(lat, lng)
			g.cells[key] = append(g.cells[key], len(g.cities))
			g.cities = append(g.cities, City{Id: id, Name: cols[1], Region: cols[2], Country: cols[3], Lat: lat, Lng: lng})
		}
		sharedGeo = g
	})
	return sharedGeo
}

func cellOf(lat, lng float64) cellKey {
	return cellKey{int(math.Floor(lat)), int(math.Floor(lng))}
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	toRad := math.Pi / 180
	dLat := (lat2 - lat1) * toRad
	dLng := (lng2 - lng1) * toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// NearestCity returns the closest city within maxCityDistance.
func NearestCity(lat, lng float64) (City, bool) {
	g := loadGeocoder()
	center := cellOf(lat, lng)
	best, bestDist := -1, maxCityDistance
	for dLat := -1; dLat <= 1; dLat++ {
		for dLng := -1; dLng <= 1; dLng++ {
			cellLng := center.lng + dLng
			if cellLng < -180 {
				cellLng += 360
			} else if cellLng >= 180 {
				cellLng -= 360
			}
			for _, i := range g.cells[cellKey{center.lat + dLat, cellLng}] {
				c := g.cities[i]
				if d := haversineMeters(lat, lng, c.Lat, c.Lng); d < bestDist {
					best, bestDist = i, d
				}
			}
		}
	}
	if best < 0 {
		return City{}, false
	}
	return g.cities[best], true
}

func CityById(id int) (City, bool) {
	g := loadGeocoder()
	i, ok := g.byId[id]
	if !ok {
		return City{}, false
	}
	return g.cities[i], true
}

type FamilyPlace struct {
	Id           int       `json:"id"`
	FamilyId     int       `json:"familyId"`
	Name         string    `json:"name"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	RadiusMeters int       `json:"radiusMeters"`
	CreatedAt    time.Time `json:"createdAt"`
}

func PackFamilyPlace(self *FamilyPlace, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Name, buf)
	vpack.Float64(&self.Latitude, buf)
	vpack.Float64(&self.Longitude, buf)
	vpack.Int(&self.RadiusMeters, buf)
	vpack.Time(&self.CreatedAt, buf)
}

var FamilyPlaceBkt = vbolt.Bucket(&cfg.Info, "family_places", vpack.FInt, PackFamilyPlace)
var FamilyPlaceByFamilyIndex = vbolt.Index(&cfg.Info, "family_place_by_family", vpack.FInt, vpack.FInt)

func GetFamilyPlaces(tx *vbolt.Tx, familyId int) (places []FamilyPlace) {
	var ids []int
	vbolt.ReadTermTargets(tx, FamilyPlaceByFamilyIndex, familyId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, FamilyPlaceBkt, ids, &places)
	sort.Slice(places, func(i, j int) bool { return places[i].Name < places[j].Name })
	return
}

// PhotoPlace is where a photo was taken, as the family sees it: one of its
// named places when the photo falls inside one, otherwise the nearest city.
type PhotoPlace struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	FamilyPlaceId int     `json:"familyPlaceId"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
}

func familyPlaceKey(id int) string { return fmt.Sprintf("f%d", id) }
func cityPlaceKey(id int) string   { return fmt.Sprintf("c%d", id) }

func resolvePlace(familyPlaces []FamilyPlace, features PhotoFeatures) (PhotoPlace, bool) {
	if !features.HasLocation {
		return PhotoPlace{}, false
	}
	place := PhotoPlace{Latitude: features.Latitude, Longitude: features.Longitude}
	bestRatio := math.MaxFloat64
	for _, fp := range familyPlaces {
		d := haversineMeters(features.Latitude, features.Longitude, fp.Latitude, fp.Longitude)
		if d > float64(fp.RadiusMeters) {
			continue
		}
		if ratio := d / float64(max(fp.RadiusMeters, 1)); ratio < bestRatio {
			bestRatio = ratio
			place.Key, place.Name, place.FamilyPlaceId = familyPlaceKey(fp.Id), fp.Name, fp.Id
		}
	}
	if place.Key != "" {
		return place, true
	}
	if city, ok := CityById(features.CityId); ok {
		place.Key, place.Name = cityPlaceKey(city.Id), city.Label()
		return place, true
	}
	place.Name = fmt.Sprintf("%.3f, %.3f", features.Latitude, features.Longitude)
	return place, true
}

// placeResolver caches each family's named places for the length of a request.
type placeResolver struct {
	tx     *vbolt.Tx
	places map[int][]FamilyPlace
}

func newPlaceResolver(tx *vbolt.Tx) *placeResolver {
	return &placeResolver{tx: tx, places: map[int][]FamilyPlace{}}
}

func (r *placeResolver) forPhoto(image Image) (PhotoPlace, bool) {
	features, ok := GetPhotoFeatures(r.tx, image.Id)
	if !ok {
		return PhotoPlace{}, false
	}
	places, cached := r.places[image.FamilyId]
	if !cached {
		places = GetFamilyPlaces(r.tx, image.FamilyId)
		r.places[image.FamilyId] = places
	}
	return resolvePlace(places, features)
}

// canSeeLocation limits coordinates and place names to members of the
// family that owns the photo; linked families see the photo but not where.
func canSeeLocation(tx *vbolt.Tx, user User, image Image) bool {
	return CanAccessFamily(tx, user, image.FamilyId, AccessView)
}

func RegisterPlaceMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListFamilyPlaces)
	vbeam.RegisterProc(app, SaveFamilyPlace)
	vbeam.RegisterProc(app, DeleteFamilyPlace)
	vbeam.RegisterProc(app, ListPhotoPlaces)
}

type ListFamilyPlacesRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type FamilyPlaceWithCount struct {
	Place      FamilyPlace `json:"place"`
	PhotoCount int         `json:"photoCount"`
}

type ListFamilyPlacesResponse struct {
	Places []FamilyPlaceWithCount `json:"places"`
}

func ListFamilyPlaces(ctx *vbeam.Context, req ListFamilyPlacesRequest) (resp ListFamilyPlacesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId := req.FamilyId
	if familyId == 0 {
		familyId = user.FamilyId
	}
	if !CanAccessFamily(ctx.Tx, user, familyId, AccessView) {
		err = ErrFamilyAccessDenied
		return
	}
	places := GetFamilyPlaces(ctx.Tx, familyId)
	counts := map[int]int{}
	for _, features := range familyFeatures(ctx.Tx, familyId) {
		if place, ok := resolvePlace(places, features); ok && place.FamilyPlaceId > 0 {
			counts[place.FamilyPlaceId]++
		}
	}
	resp.Places = make([]FamilyPlaceWithCount, 0, len(places))
	for _, p := range places {
		resp.Places = append(resp.Places, FamilyPlaceWithCount{Place: p, PhotoCount: counts[p.Id]})
	}
	return
}

type SaveFamilyPlaceRequest struct {
	Id           int    `json:"id,omitempty"`
	PhotoId      int    `json:"photoId,omitempty"`
	Name         string `json:"name"`
	RadiusMeters int    `json:"radiusMeters"`
}

type SaveFamilyPlaceResponse struct {
	Place FamilyPlace `json:"place"`
}

// SaveFamilyPlace renames or resizes an existing place (Id), or creates one
// centred on where a photo was taken (PhotoId).
func SaveFamilyPlace(ctx *vbeam.Context, req SaveFamilyPlaceRequest) (resp SaveFamilyPlaceResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > maxPlaceNameLength {
		err = fmt.Errorf("Place names need 1 to %d characters", maxPlaceNameLength)
		return
	}
	radius := req.RadiusMeters
	if radius <= 0 {
		radius = defaultPlaceRadius
	}
	radius = min(radius, maxFamilyPlaceRadius)

	vbeam.UseWriteTx(ctx)
	var place FamilyPlace
	if req.Id > 0 {
		vbolt.Read(ctx.Tx, FamilyPlaceBkt, req.Id, &place)
		if place.Id == 0 || !CanAccessFamily(ctx.Tx, user, place.FamilyId, AccessContribute) {
			err = errors.New("Place not found")
			return
		}
	} else {
		photo := GetImageById(ctx.Tx, req.PhotoId)
		if photo.Id == 0 || !CanAccessFamily(ctx.Tx, user, photo.FamilyId, AccessContribute) {
			err = errors.New("Photo not found")
			return
		}
		features, ok := GetPhotoFeatures(ctx.Tx, photo.Id)
		if !ok || !features.HasLocation {
			err = errors.New("That photo has no location")
			return
		}
		place = FamilyPlace{
			Id:        vbolt.NextIntId(ctx.Tx, FamilyPlaceBkt),
			FamilyId:  photo.FamilyId,
			Latitude:  features.Latitude,
			Longitude: features.Longitude,
			CreatedAt: time.Now(),
		}
	}
	place.Name = name
	place.RadiusMeters = radius
	vbolt.Write(ctx.Tx, FamilyPlaceBkt, place.Id, &place)
	vbolt.SetTargetSingleTerm(ctx.Tx, FamilyPlaceByFamilyIndex, place.Id, place.FamilyId)
	resp.Place = place
	vbolt.TxCommit(ctx.Tx)
	return
}

type DeleteFamilyPlaceRequest struct {
	Id int `json:"id"`
}

func DeleteFamilyPlace(ctx *vbeam.Context, req DeleteFamilyPlaceRequest) (resp Empty, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	vbeam.UseWriteTx(ctx)
	var place FamilyPlace
	vbolt.Read(ctx.Tx, FamilyPlaceBkt, req.Id, &place)
	if place.Id == 0 || !CanAccessFamily(ctx.Tx, user, place.FamilyId, AccessContribute) {
		err = errors.New("Place not found")
		return
	}
	vbolt.Delete(ctx.Tx, FamilyPlaceBkt, place.Id)
	vbolt.DeleteTargetTerms(ctx.Tx, FamilyPlaceByFamilyIndex, place.Id)
	vbolt.TxCommit(ctx.Tx)
	return
}

type PlaceCount struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ListPhotoPlacesResponse struct {
	Places []PlaceCount `json:"places"`
}

// ListPhotoPlaces counts photos per resolved place across the families the
// user belongs to, for the photo filter.
func ListPhotoPlaces(ctx *vbeam.Context, req Empty) (resp ListPhotoPlacesResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	byKey := map[string]*PlaceCount{}
	for _, familyId := range familiesVisibleTo(ctx.Tx, user) {
		places := GetFamilyPlaces(ctx.Tx, familyId)
		for _, features := range familyFeatures(ctx.Tx, familyId) {
			place, ok := resolvePlace(places, features)
			if !ok || place.Key == "" {
				continue
			}
			if byKey[place.Key] == nil {
				byKey[place.Key] = &PlaceCount{Key: place.Key, Name: place.Name}
			}
			byKey[place.Key].Count++
		}
	}
	resp.Places = make([]PlaceCount, 0, len(byKey))
	for _, pc := range byKey {
		resp.Places = append(resp.Places, *pc)
	}
	sort.Slice(resp.Places, func(i, j int) bool {
		a, b := resp.Places[i], resp.Places[j]
		if (a.Key[0] == 'f') != (b.Key[0] == 'f') {
			return a.Key[0] == 'f'
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Name < b.Name
	})
	return
}
