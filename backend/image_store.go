package backend

import (
	"time"

	"go.hasen.dev/vbolt"
)

// The image store is the only code that writes an Image, the people on it or
// its transcript. Each call leaves every index derived from them current.

func createImage(tx *vbolt.Tx, image *Image) {
	if image.Id == 0 {
		image.Id = vbolt.NextIntId(tx, ImagesBkt)
	}
	vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, image.FamilyId)
	saveImage(tx, image)
}

func saveImage(tx *vbolt.Tx, image *Image) {
	writeImage(tx, image)
	ReindexPhotoDates(tx, image.Id)
}

func deleteImage(tx *vbolt.Tx, imageId int) {
	deleteVideoTranscriptTx(tx, imageId)
	for _, pp := range GetPhotoPersonsByPhoto(tx, imageId) {
		deletePhotoPersonRow(tx, pp)
	}
	vbolt.Delete(tx, ImagesBkt, imageId)
	vbolt.DeleteTargetTerms(tx, ImageByFamilyIndex, imageId)
	vbolt.DeleteTargetTerms(tx, ImageByAnchorIndex, imageId)
	ReindexPhotoDates(tx, imageId)
}

func AddPersonToPhoto(tx *vbolt.Tx, photoId int, personId int, familyId int) (photoPersonId int) {
	pp := PhotoPerson{PhotoId: photoId, PersonId: personId, FamilyId: familyId, CreatedAt: time.Now()}
	insertPhotoPerson(tx, &pp)
	return pp.Id
}

func insertPhotoPerson(tx *vbolt.Tx, pp *PhotoPerson) {
	if pp.Id == 0 {
		pp.Id = vbolt.NextIntId(tx, PhotoPersonBkt)
	}
	writePhotoPersonRow(tx, pp)
	ReindexPhotoDates(tx, pp.PhotoId)
}

func RemovePersonFromPhoto(tx *vbolt.Tx, photoId int, personId int) {
	for _, pp := range GetPhotoPersonsByPhoto(tx, photoId) {
		if pp.PersonId == personId {
			deletePhotoPerson(tx, pp)
			return
		}
	}
}

func deletePhotoPerson(tx *vbolt.Tx, pp PhotoPerson) {
	deletePhotoPersonRow(tx, pp)
	ReindexPhotoDates(tx, pp.PhotoId)
}

// movePhotoPerson puts toPersonId in pp's place. A photo that already has
// toPersonId just loses pp, and moved is false.
func movePhotoPerson(tx *vbolt.Tx, pp PhotoPerson, toPersonId int) (moved bool) {
	for _, existing := range GetPhotoPersonsByPhoto(tx, pp.PhotoId) {
		if existing.Id != pp.Id && existing.PersonId == toPersonId {
			deletePhotoPerson(tx, pp)
			return false
		}
	}
	pp.PersonId = toPersonId
	writePhotoPersonRow(tx, &pp)
	ReindexPhotoDates(tx, pp.PhotoId)
	return true
}

func setPhotoPersonAutoTagged(tx *vbolt.Tx, pp PhotoPerson, auto bool) {
	pp.AutoTagged = auto
	writePhotoPersonRow(tx, &pp)
	ReindexPhotoDates(tx, pp.PhotoId)
}

func writePhotoPersonRow(tx *vbolt.Tx, pp *PhotoPerson) {
	vbolt.Write(tx, PhotoPersonBkt, pp.Id, pp)
	indexPhotoPersonRow(tx, *pp)
}

func indexPhotoPersonRow(tx *vbolt.Tx, pp PhotoPerson) {
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, pp.Id, pp.PhotoId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, pp.Id, pp.PersonId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, pp.Id, pp.FamilyId)
}

func deletePhotoPersonRow(tx *vbolt.Tx, pp PhotoPerson) {
	vbolt.Delete(tx, PhotoPersonBkt, pp.Id)
	vbolt.DeleteTargetTerms(tx, PhotoPersonByPhotoIndex, pp.Id)
	vbolt.DeleteTargetTerms(tx, PhotoPersonByPersonIndex, pp.Id)
	vbolt.DeleteTargetTerms(tx, PhotoPersonByFamilyIndex, pp.Id)
}

func writeVideoTranscriptTx(tx *vbolt.Tx, t *VideoTranscript) {
	vbolt.Write(tx, VideoTranscriptBkt, t.ImageId, t)
	dropStaleSpeechVectorsTx(tx, *t)
	ReindexPhotoDates(tx, t.ImageId)
}

// RebuildImageIndexes recomputes every index the store keeps from the stored
// images, photo people and transcripts.
func RebuildImageIndexes(tx *vbolt.Tx) {
	var people []PhotoPerson
	vbolt.IterateAll(tx, PhotoPersonBkt, func(_ int, pp PhotoPerson) bool {
		people = append(people, pp)
		return true
	})
	for _, pp := range people {
		indexPhotoPersonRow(tx, pp)
	}

	var images []Image
	vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
		images = append(images, image)
		return true
	})
	for _, image := range images {
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, image.FamilyId)
		vbolt.SetTargetTermsPlain(tx, ImageByAnchorIndex, image.Id, anchorTerm(image.RecordTime))
		ReindexPhotoDates(tx, image.Id)
	}
}
