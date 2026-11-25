package backend

import (
	"fmt"
	"time"

	"go.hasen.dev/vbolt"
)

type ExportBookItem struct {
	Kind     string `json:"kind"`
	SourceId int    `json:"sourceId"`
	PhotoId  int    `json:"photoId,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Pinned   bool   `json:"pinned,omitempty"`
}

type ExportBook struct {
	Id           int              `json:"id"`
	PersonId     int              `json:"personId"`
	PersonIds    []int            `json:"personIds,omitempty"`
	Preset       string           `json:"preset"`
	Title        string           `json:"title"`
	StartDate    time.Time        `json:"startDate"`
	EndDate      time.Time        `json:"endDate"`
	CoverPhotoId int              `json:"coverPhotoId,omitempty"`
	Density      string           `json:"density"`
	Categories   []string         `json:"categories,omitempty"`
	Match        string           `json:"match,omitempty"`
	Introduction string           `json:"introduction,omitempty"`
	Letter       string           `json:"letter,omitempty"`
	Signature    string           `json:"signature,omitempty"`
	ShowGrowth   bool             `json:"showGrowth"`
	Items        []ExportBookItem `json:"items"`
	Excluded     []ExportBookItem `json:"excluded,omitempty"`
}

var bookItemKindNames = map[BookItemKind]string{
	BookItemMilestone: "milestone",
	BookItemPhoto:     "photo",
	BookItemInterview: "interview",
}

func exportBookItems(items []BookItem) []ExportBookItem {
	out := make([]ExportBookItem, 0, len(items))
	for _, item := range items {
		out = append(out, ExportBookItem{
			Kind:     bookItemKindNames[item.Kind],
			SourceId: item.SourceId,
			PhotoId:  item.PhotoId,
			Caption:  item.Caption,
			Pinned:   item.Pinned,
		})
	}
	return out
}

func buildBookExport(tx *vbolt.Tx, familyId int) []ExportBook {
	books := []ExportBook{}
	for _, book := range GetFamilyBooks(tx, familyId) {
		books = append(books, ExportBook{
			Id:           book.Id,
			PersonId:     book.PersonId,
			PersonIds:    book.PersonIds,
			Preset:       book.Preset,
			Title:        book.Title,
			StartDate:    book.StartDate,
			EndDate:      book.EndDate,
			CoverPhotoId: book.CoverPhotoId,
			Density:      book.Density,
			Categories:   book.Categories,
			Match:        book.Match,
			Introduction: book.Introduction,
			Letter:       book.Letter,
			Signature:    book.Signature,
			ShowGrowth:   book.ShowGrowth,
			Items:        exportBookItems(book.Items),
			Excluded:     exportBookItems(book.Excluded),
		})
	}
	return books
}

type bookImport struct {
	familyId int
	userId   int
	ledger   *archiveLedger
}

func (in bookImport) mapItem(item ExportBookItem) (BookItem, bool) {
	out := BookItem{SourceId: 0, Caption: item.Caption, Pinned: item.Pinned}
	var ok bool
	switch item.Kind {
	case "milestone":
		out.Kind = BookItemMilestone
		out.SourceId, ok = in.ledger.moments[item.SourceId]
		if item.PhotoId != 0 {
			out.PhotoId = in.ledger.photos[item.PhotoId]
		}
	case "photo":
		out.Kind = BookItemPhoto
		out.SourceId, ok = in.ledger.photos[item.SourceId]
	case "interview":
		out.Kind = BookItemInterview
		out.SourceId, ok = in.ledger.interviews[item.SourceId]
	}
	return out, ok && out.SourceId != 0
}

func (in bookImport) mapPeople(source ExportBook) []int {
	ids := source.PersonIds
	if len(ids) == 0 {
		ids = []int{source.PersonId}
	}
	mapped := []int{}
	seen := map[int]bool{}
	for _, id := range ids {
		if newId, ok := in.ledger.people[id]; ok && !seen[newId] {
			seen[newId] = true
			mapped = append(mapped, newId)
		}
	}
	return mapped
}

func bookAlreadyImported(tx *vbolt.Tx, personId int, source ExportBook) bool {
	for _, existing := range GetPersonBooks(tx, personId) {
		if existing.Preset == source.Preset && existing.Title == source.Title && existing.StartDate.Equal(source.StartDate) {
			return true
		}
	}
	return false
}

func importBooks(tx *vbolt.Tx, books []ExportBook, in bookImport) (imported, skipped int, warnings []string) {
	now := time.Now()
	for _, source := range books {
		personIds := in.mapPeople(source)
		if len(personIds) == 0 || bookAlreadyImported(tx, personIds[0], source) {
			skipped++
			continue
		}

		book := Book{
			Id:           vbolt.NextIntId(tx, BookBkt),
			FamilyId:     in.familyId,
			PersonIds:    personIds,
			Preset:       source.Preset,
			Title:        source.Title,
			StartDate:    source.StartDate,
			EndDate:      source.EndDate,
			CoverPhotoId: in.ledger.photos[source.CoverPhotoId],
			Density:      cleanDensity(source.Density),
			Categories:   cleanCategories(source.Categories),
			Match:        cleanMatch(source.Match),
			Introduction: source.Introduction,
			Letter:       source.Letter,
			Signature:    source.Signature,
			ShowGrowth:   source.ShowGrowth,
			Items:        []BookItem{},
			Excluded:     []BookItem{},
			Revision:     1,
			CreatedBy:    in.userId,
			CreatedAt:    now,
			UpdatedAt:    now,
			ReviewedAt:   now,
		}
		if book.Preset == "" {
			book.Preset = BookPresetFirstYear
		}

		unmatched := 0
		for _, item := range source.Items {
			mapped, ok := in.mapItem(item)
			if !ok {
				unmatched++
				continue
			}
			if item.PhotoId != 0 && mapped.PhotoId == 0 {
				unmatched++
			}
			book.Items = append(book.Items, mapped)
		}
		for _, item := range source.Excluded {
			if mapped, ok := in.mapItem(item); ok {
				book.Excluded = append(book.Excluded, BookItem{Kind: mapped.Kind, SourceId: mapped.SourceId})
			}
		}
		if source.CoverPhotoId != 0 && book.CoverPhotoId == 0 {
			unmatched++
		}

		writeBookTx(tx, book)
		imported++
		if unmatched > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"Book %q: %d of its photos or records were not in this import and were left out", book.Title, unmatched))
		}
	}
	return
}
