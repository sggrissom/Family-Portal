package backend

import "testing"

func TestArtworkKeepsItsPhotosOnThePerson(t *testing.T) {
	fx := setupResultsFixture(t)
	day := "2019-03-12"
	drawing := fx.addPhoto(t, fx.familyId, "dinosaur-family.jpg")

	added, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "Drew the whole family as dinosaurs", Category: "artwork",
		Context: "only quotes keep this", InputType: "date", MilestoneDate: &day, PhotoIds: []int{drawing.Id},
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.Milestone.Category != "artwork" || added.Milestone.Context != "" {
		t.Errorf("added %+v, want artwork with no context", added.Milestone)
	}

	person, err := callAs(t, fx, GetPerson, GetPersonRequest{Id: fx.alice.Id})
	if err != nil {
		t.Fatal(err)
	}
	var found *Milestone
	for i := range person.Milestones {
		if person.Milestones[i].Id == added.Milestone.Id {
			found = &person.Milestones[i]
		}
	}
	if found == nil || len(found.PhotoIds) != 1 || found.PhotoIds[0] != drawing.Id {
		t.Fatalf("person's milestones %+v, want the artwork with its drawing", person.Milestones)
	}

	byCategory, _ := callAs(t, fx, SearchMilestones, SearchMilestonesRequest{Query: "dinosaurs"})
	if len(byCategory.Milestones) != 1 || byCategory.Milestones[0].Category != "artwork" {
		t.Errorf("search found %v, want the artwork", byCategory.Milestones)
	}
}

func TestUnknownCategoryIsStillRejected(t *testing.T) {
	fx := setupResultsFixture(t)
	day := "2019-03-12"
	_, err := callAs(t, fx, AddMilestone, AddMilestoneRequest{
		PersonId: fx.alice.Id, Description: "Report card", Category: "documents",
		InputType: "date", MilestoneDate: &day,
	})
	if err == nil {
		t.Error("an unknown category was accepted")
	}
}
