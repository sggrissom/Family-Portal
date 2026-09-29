package backend

import (
	"bytes"
	"context"
	"family/cfg"
	"family/vision"
	"image"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// Each analyzer's version is stored with its result. Bumping one makes the
// startup sweep rerun that analyzer, and only that one, over every photo.
const (
	hashAnalyzerVersion    = 1
	qualityAnalyzerVersion = 1
	placeAnalyzerVersion   = 1
)

const (
	burstWindow       = 2 * time.Minute
	burstMaxDistance  = 12
	duplicateDistance = 3
	hashChunks        = 4
)

type PhotoFeatures struct {
	PhotoId  int
	FamilyId int

	Hash        uint64
	HashVersion int
	GroupId     int

	Sharpness      float64
	Exposure       float64
	Quality        float64
	QualityVersion int

	HasLocation  bool
	Latitude     float64
	Longitude    float64
	CityId       int
	PlaceVersion int
}

func PackPhotoFeatures(self *PhotoFeatures, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.FUInt64(&self.Hash, buf)
	vpack.Int(&self.HashVersion, buf)
	vpack.Int(&self.GroupId, buf)
	vpack.Float64(&self.Sharpness, buf)
	vpack.Float64(&self.Exposure, buf)
	vpack.Float64(&self.Quality, buf)
	vpack.Int(&self.QualityVersion, buf)
	vpack.Bool(&self.HasLocation, buf)
	vpack.Float64(&self.Latitude, buf)
	vpack.Float64(&self.Longitude, buf)
	vpack.Int(&self.CityId, buf)
	vpack.Int(&self.PlaceVersion, buf)
}

var PhotoFeaturesBkt = vbolt.Bucket(&cfg.Info, "photo_features", vpack.FInt, PackPhotoFeatures)
var FeaturesByFamilyIndex = vbolt.Index(&cfg.Info, "features_by_family", vpack.FInt, vpack.FInt)
var FeaturesByGroupIndex = vbolt.Index(&cfg.Info, "features_by_group", vpack.FInt, vpack.FInt)
var FeaturesByHashChunkIndex = vbolt.Index(&cfg.Info, "features_by_hash_chunk", vpack.FInt, vpack.FInt)

func GetPhotoFeatures(tx *vbolt.Tx, photoId int) (features PhotoFeatures, ok bool) {
	ok = vbolt.Read(tx, PhotoFeaturesBkt, photoId, &features)
	return
}

func readFeatures(tx *vbolt.Tx, index *vbolt.IndexInfo[int, int, uint16], term int) (features []PhotoFeatures) {
	var ids []int
	vbolt.ReadTermTargets(tx, index, term, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoFeaturesBkt, ids, &features)
	return
}

func familyFeatures(tx *vbolt.Tx, familyId int) []PhotoFeatures {
	return readFeatures(tx, FeaturesByFamilyIndex, familyId)
}

func hashChunkTerms(hash uint64) []int {
	terms := make([]int, hashChunks)
	for i := range terms {
		terms[i] = i<<16 | int((hash>>(16*i))&0xFFFF)
	}
	return terms
}

func writeFeaturesTx(tx *vbolt.Tx, f *PhotoFeatures) {
	vbolt.Write(tx, PhotoFeaturesBkt, f.PhotoId, f)
	vbolt.SetTargetSingleTerm(tx, FeaturesByFamilyIndex, f.PhotoId, f.FamilyId)
	if f.HashVersion > 0 {
		vbolt.SetTargetTermsPlain(tx, FeaturesByHashChunkIndex, f.PhotoId, hashChunkTerms(f.Hash))
	} else {
		vbolt.DeleteTargetTerms(tx, FeaturesByHashChunkIndex, f.PhotoId)
	}
	if f.GroupId > 0 {
		vbolt.SetTargetSingleTerm(tx, FeaturesByGroupIndex, f.PhotoId, f.GroupId)
	} else {
		vbolt.DeleteTargetTerms(tx, FeaturesByGroupIndex, f.PhotoId)
	}
}

func deletePhotoFeaturesTx(tx *vbolt.Tx, photoId int) {
	vbolt.Delete(tx, PhotoFeaturesBkt, photoId)
	vbolt.DeleteTargetTerms(tx, FeaturesByFamilyIndex, photoId)
	vbolt.DeleteTargetTerms(tx, FeaturesByGroupIndex, photoId)
	vbolt.DeleteTargetTerms(tx, FeaturesByHashChunkIndex, photoId)
}

func featuresOutdated(f PhotoFeatures, found bool) bool {
	return !found || f.HashVersion < hashAnalyzerVersion ||
		f.QualityVersion < qualityAnalyzerVersion || f.PlaceVersion < placeAnalyzerVersion
}

func similarPhotos(a, b PhotoFeatures, aDate, bDate time.Time) bool {
	d := vision.HammingDistance(a.Hash, b.Hash)
	if d <= duplicateDistance {
		return true
	}
	gap := aDate.Sub(bDate)
	if gap < 0 {
		gap = -gap
	}
	return d <= burstMaxDistance && gap <= burstWindow
}

// linkSimilarTx puts the photo in a group with every similar photo in its
// family, merging groups it bridges. A group is named by its lowest photo id.
func linkSimilarTx(tx *vbolt.Tx, f *PhotoFeatures, date time.Time) {
	candidates := map[int]bool{}
	stream := newPhotoStream(ImageByFamilyDateIndex, f.FamilyId, photoKey{seconds: date.Add(burstWindow).Unix(), id: math.MaxInt})
	floor := date.Add(-burstWindow).Unix()
	for stream.peek(tx) {
		img := stream.pop()
		if img.PhotoDate.Unix() < floor {
			break
		}
		candidates[img.Id] = true
	}
	for _, term := range hashChunkTerms(f.Hash) {
		var ids []int
		vbolt.ReadTermTargets(tx, FeaturesByHashChunkIndex, term, &ids, vbolt.Window{})
		for _, id := range ids {
			candidates[id] = true
		}
	}
	delete(candidates, f.PhotoId)

	groups := map[int]bool{}
	var loose []PhotoFeatures
	for id := range candidates {
		other, ok := GetPhotoFeatures(tx, id)
		if !ok || other.HashVersion == 0 || other.FamilyId != f.FamilyId {
			continue
		}
		otherImage := GetImageById(tx, id)
		if otherImage.Id == 0 || otherImage.Status != 0 || !similarPhotos(*f, other, date, otherImage.PhotoDate) {
			continue
		}
		if other.GroupId > 0 {
			groups[other.GroupId] = true
		} else {
			loose = append(loose, other)
		}
	}
	if len(groups) == 0 && len(loose) == 0 {
		f.GroupId = 0
		return
	}

	target := f.PhotoId
	for g := range groups {
		target = min(target, g)
	}
	for _, other := range loose {
		target = min(target, other.PhotoId)
	}
	for g := range groups {
		if g == target {
			continue
		}
		for _, member := range readFeatures(tx, FeaturesByGroupIndex, g) {
			member.GroupId = target
			writeFeaturesTx(tx, &member)
		}
	}
	for _, other := range loose {
		other.GroupId = target
		writeFeaturesTx(tx, &other)
	}
	f.GroupId = target
}

type photoGroup struct {
	cover   int
	members []int
}

// groupOf returns the photo's similar-photo group, with the best-quality
// photo as its cover, or false when the photo stands alone.
func groupOf(tx *vbolt.Tx, groupId int, cache map[int]photoGroup) (photoGroup, bool) {
	if groupId == 0 {
		return photoGroup{}, false
	}
	if g, ok := cache[groupId]; ok {
		return g, len(g.members) > 1
	}
	var members []PhotoFeatures
	for _, f := range readFeatures(tx, FeaturesByGroupIndex, groupId) {
		if img := GetImageById(tx, f.PhotoId); img.Id != 0 && img.Status == 0 {
			members = append(members, f)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].PhotoId < members[j].PhotoId })
	g := photoGroup{}
	best := -1.0
	for _, m := range members {
		g.members = append(g.members, m.PhotoId)
		if m.Quality > best {
			best, g.cover = m.Quality, m.PhotoId
		}
	}
	cache[groupId] = g
	return g, len(g.members) > 1
}

func featureSourceImage(img Image) (image.Image, error) {
	base := strings.TrimSuffix(filepath.Join(cfg.StaticDir, img.FilePath), filepath.Ext(img.FilePath))
	for _, candidate := range []string{base + "_medium.jpg", base + "_xlarge.jpg", base + ".jpg"} {
		if decoded, err := imaging.Open(candidate); err == nil {
			return decoded, nil
		}
	}
	return imaging.Open(getOriginalPhotoPath(img), imaging.AutoOrientation(true))
}

func exifLocation(path string) (lat, lng float64, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false
	}
	lat, lng, err = x.LatLong()
	if err != nil || math.IsNaN(lat) || math.IsNaN(lng) || (lat == 0 && lng == 0) ||
		math.Abs(lat) > 90 || math.Abs(lng) > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

func analyzePhotoFeatures(db *vbolt.DB, photoId int) {
	var img Image
	var current PhotoFeatures
	var found bool
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		img = GetImageById(tx, photoId)
		current, found = GetPhotoFeatures(tx, photoId)
	})
	if img.Id == 0 || img.Status != 0 || !featuresOutdated(current, found) {
		return
	}

	next := current
	next.PhotoId, next.FamilyId = img.Id, img.FamilyId
	rehashed := false
	if next.HashVersion < hashAnalyzerVersion || next.QualityVersion < qualityAnalyzerVersion {
		decoded, err := featureSourceImage(img)
		if err != nil {
			log.Printf("[FEATURES] Could not open photo %d: %v", img.Id, err)
		} else {
			if next.HashVersion < hashAnalyzerVersion {
				next.Hash = vision.DHash(decoded)
				rehashed = true
			}
			q := vision.MeasureQuality(decoded)
			next.Sharpness, next.Exposure, next.Quality = q.Sharpness, q.Exposure, q.Score
		}
		next.HashVersion, next.QualityVersion = hashAnalyzerVersion, qualityAnalyzerVersion
	}
	if next.PlaceVersion < placeAnalyzerVersion {
		next.HasLocation, next.Latitude, next.Longitude, next.CityId = false, 0, 0, 0
		if lat, lng, ok := exifLocation(getOriginalPhotoPath(img)); ok {
			next.HasLocation, next.Latitude, next.Longitude = true, lat, lng
			if city, ok := NearestCity(lat, lng); ok {
				next.CityId = city.Id
			}
		}
		next.PlaceVersion = placeAnalyzerVersion
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, photoId)
		if img.Id == 0 {
			return
		}
		if rehashed {
			linkSimilarTx(tx, &next, img.PhotoDate)
		}
		writeFeaturesTx(tx, &next)
		vbolt.TxCommit(tx)
	})
}

func photosNeedingFeatures(db *vbolt.DB) (ids []int) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, img Image) bool {
			if img.Status == 0 {
				f, found := GetPhotoFeatures(tx, img.Id)
				if featuresOutdated(f, found) {
					ids = append(ids, img.Id)
				}
			}
			return true
		})
	})
	if len(ids) > 0 {
		LogInfo(LogCategoryWorker, "Queued photos for feature analysis", map[string]interface{}{"count": len(ids)})
	}
	return
}

var globalFeaturesWorker *backlogWorker[int]

func InitializeFeaturesWorker(db *vbolt.DB) {
	if globalFeaturesWorker != nil {
		return
	}
	globalFeaturesWorker = newBacklogWorker("Photo features worker", func(photoId int) error {
		analyzePhotoFeatures(db, photoId)
		return nil
	})
	globalFeaturesWorker.run(func() []int { return photosNeedingFeatures(db) })
	LogInfo(LogCategoryWorker, "Photo features worker started")
}

func QueuePhotoFeatures(photoId int) {
	if globalFeaturesWorker != nil {
		globalFeaturesWorker.add(photoId)
	}
}

func FeaturesQueueLength() int {
	if globalFeaturesWorker == nil {
		return 0
	}
	return globalFeaturesWorker.length()
}

func StopFeaturesWorker(ctx context.Context) bool {
	if globalFeaturesWorker == nil {
		return true
	}
	return globalFeaturesWorker.stopWait(ctx)
}
