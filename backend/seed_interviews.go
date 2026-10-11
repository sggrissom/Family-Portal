package backend

import "fmt"

var seedInterviewAnswers = []struct {
	key     string
	answers []string
}{
	{"fav-color", []string{"red", "blue", "purple", "green", "yellow", "pink", "orange"}},
	{"fav-food", []string{"mac and cheese", "pizza", "tacos", "sushi", "spaghetti", "chicken nuggets", "pancakes"}},
	{"fav-snack", []string{"goldfish", "apple slices", "popcorn", "pretzels", "cheese sticks", "grapes"}},
	{"fav-animal", []string{"dog", "shark", "octopus", "horse", "cat", "penguin", "red panda"}},
	{"fav-show", []string{"Bluey", "Wild Kratts", "Octonauts", "Paw Patrol", "The Great British Baking Show"}},
	{"fav-book", []string{"Goodnight Moon", "Dragons Love Tacos", "Magic Tree House", "Dog Man", "Wings of Fire", "Harry Potter"}},
	{"fav-holiday", []string{"Christmas", "Halloween", "my birthday", "Fourth of July", "Thanksgiving"}},
	{"good-at", []string{"running fast", "drawing", "building Legos", "reading", "making people laugh", "soccer"}},
	{"grow-up", []string{"a firefighter", "a paleontologist", "a doctor", "an astronaut", "a chef", "an artist", "a vet"}},
	{"best-friend", []string{"Theo", "Lily", "Max", "Ava", "Noor", "Sam"}},
	{"laugh", []string{"tickles", "Dad's jokes", "the dog", "silly faces", "fart noises"}},
	{"superpower", []string{"flying", "being invisible", "super speed", "teleporting", "talking to animals"}},
	{"million", []string{"a pony", "a candy store", "a house for everyone", "a rocket", "a pool"}},
}

// interviews gives a child birthday interviews for the last few years, with
// answers that sometimes hold and sometimes change from one year to the next.
func (s *seeder) interviews(person Person) {
	if person.Id == 0 || person.IsPregnancy || s.err != nil {
		return
	}
	age := ageYearsAt(person.Birthday, s.now)
	var batch []HistoricalInterview
	previous := map[string]string{}
	for year := max(3, age-3); year <= age; year++ {
		date := dayStart(person.Birthday).AddDate(year, 0, s.rng.Intn(10))
		if date.After(s.now) {
			break
		}
		in := HistoricalInterview{InputType: "date", Date: date.Format("2006-01-02")}
		for _, q := range seedInterviewAnswers {
			if s.rng.Float64() < 0.15 {
				continue
			}
			answer, kept := previous[q.key]
			if !kept || s.rng.Float64() < 0.45 {
				answer = q.answers[s.rng.Intn(len(q.answers))]
			}
			previous[q.key] = answer
			in.Answers = append(in.Answers, ImportAnswer{Ref: "key:" + q.key, Answer: answer})
		}
		batch = append(batch, in)
	}
	if len(batch) == 0 {
		return
	}
	saved, _, err := importInterviewsTx(s.tx, person, batch)
	if err != nil {
		s.fail(fmt.Errorf("interviews for %s: %w", person.Name, err))
		return
	}
	s.sum.Interviews += len(saved)
}
