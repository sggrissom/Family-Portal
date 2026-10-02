package backend

import (
	"fmt"
	"sort"
	"time"

	"go.hasen.dev/vbolt"
)

type seedBook struct {
	creator    User
	preset     string
	people     []Person
	start, end time.Time
	content    BookContent
	captions   map[int]string
}

func seedItemCategory(m Milestone) string {
	switch m.Category {
	case "quote":
		return "quotes"
	case "artwork":
		return "artwork"
	}
	return "milestones"
}

// book picks everything in range the way a "detailed" draft would, without the
// photo budget: the seed has few enough photos that all of them fit. The
// content goes through applyBookContent so a seeded book is held to the same
// checks as one saved from the editor.
func (s *seeder) book(spec seedBook) Book {
	if s.err != nil || len(spec.people) == 0 {
		return Book{}
	}
	familyId := spec.people[0].FamilyId
	sources := bookSourcesTx(s.tx, spec.creator, familyId, spec.people, spec.start, readLimit(spec.preset, spec.end), nil)

	categories := map[string]bool{}
	for _, c := range spec.content.Categories {
		categories[c] = true
	}
	wants := func(c string) bool { return len(categories) == 0 || categories[c] }

	type dated struct {
		day  time.Time
		item BookItem
	}
	var picked []dated
	for _, m := range sources.Milestones {
		if wants(seedItemCategory(m)) {
			picked = append(picked, dated{m.MilestoneDate, BookItem{Kind: BookItemMilestone, SourceId: m.Id}})
		}
	}
	if wants("photos") {
		for _, p := range sources.Photos {
			if len(sources.PhotoPeople[p.Id]) > 0 {
				picked = append(picked, dated{p.PhotoDate, BookItem{Kind: BookItemPhoto, SourceId: p.Id, Caption: spec.captions[p.Id]}})
			}
		}
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].day.Before(picked[j].day) })

	content := spec.content
	content.Items = []BookItem{}
	for _, d := range picked {
		content.Items = append(content.Items, d.item)
	}
	content.ReviewedAt = s.now

	book := Book{
		FamilyId:   familyId,
		Preset:     spec.preset,
		StartDate:  spec.start,
		EndDate:    spec.end,
		CreatedBy:  spec.creator.Id,
		CreatedAt:  s.now,
		UpdatedAt:  s.now,
		ReviewedAt: s.now,
		Revision:   1,
	}
	for _, p := range spec.people {
		book.PersonIds = append(book.PersonIds, p.Id)
	}
	if err := applyBookContent(s.tx, spec.creator, &book, spec.people, content); err != nil {
		s.fail(fmt.Errorf("book %q: %w", spec.content.Title, err))
		return Book{}
	}
	book.Id = vbolt.NextIntId(s.tx, BookBkt)
	writeBookTx(s.tx, book)
	s.sum.Books++
	return book
}

type seedBookPhotos struct {
	newborn, baptism, ballet, fieldDay, bubbles int
}

// books covers each preset once, plus a custom book narrowed to quotes and
// artwork, so the shelf and every chapter layout have something to show.
func (s *seeder) books(mom, dad User, kids []Person, maeve, esme Person, photos seedBookPhotos) {
	start, end := firstYearRange(maeve.Birthday)
	s.book(seedBook{
		creator: mom,
		preset:  BookPresetFirstYear,
		people:  []Person{maeve},
		start:   start,
		end:     end,
		captions: map[int]string{
			photos.baptism: "Four generations in one pew, and one very sleepy guest of honour.",
		},
		content: BookContent{
			Title:        "Maeve's first year",
			CoverPhotoId: photos.newborn,
			Density:      "detailed",
			Introduction: "You arrived on a rainy Wednesday in April, a week later than anyone had planned, and the house has been louder and happier ever since. This is your first year, as best we managed to write it down.",
			Letter:       "Dear Maeve,\n\nYou won't remember any of this, so we did the remembering for you. Your brothers and sisters fought over who got to hold you first. Grandma knitted three blankets before you could sit up. You laughed at the dog every single time.\n\nWe love you more than this book can hold.",
			Signature:    "Mom and Dad",
			ShowGrowth:   true,
		},
	})

	today := dayStart(s.now)
	pastStart, pastEnd := today.AddDate(-1, 0, 1), today.AddDate(0, 0, 1)

	s.book(seedBook{
		creator: mom,
		preset:  BookPresetYear,
		people:  []Person{esme},
		start:   pastStart,
		end:     pastEnd,
		captions: map[int]string{
			photos.ballet: "Five minutes before the solo that won Regionals.",
		},
		content: BookContent{
			Title:        "Esme's year",
			CoverPhotoId: photos.ballet,
			Density:      "balanced",
			Introduction: "The year of pointe shoes, early mornings at the studio, and a first-place ribbon that now lives on the fridge.",
			ShowGrowth:   true,
		},
	})

	s.book(seedBook{
		creator: dad,
		preset:  BookPresetFamilyYear,
		people:  kids,
		start:   pastStart,
		end:     pastEnd,
		content: BookContent{
			Title:        "Our year",
			CoverPhotoId: photos.bubbles,
			Density:      "brief",
			Introduction: "Five kids, three sports, one more on the way. Here is the year we had together.",
			Signature:    "Owen",
		},
	})

	s.book(seedBook{
		creator: mom,
		preset:  BookPresetCustom,
		people:  kids,
		start:   dayStart(kids[0].Birthday),
		end:     pastEnd,
		content: BookContent{
			Title:        "Things they said and made",
			CoverPhotoId: photos.fieldDay,
			Density:      "detailed",
			Categories:   []string{"artwork", "quotes"},
			Introduction: "The fridge ran out of room years ago. These are the drawings, the clay pots, and the lines we couldn't stop repeating.",
		},
	})
}
