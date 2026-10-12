package backend

type defaultInterviewQuestion struct {
	Key     string
	Section string
	Text    string
	MinAge  int
	MaxAge  int
}

const (
	sectionFavorites = "Favorites"
	sectionAboutMe   = "About me"
	sectionPeople    = "People"
	sectionThisYear  = "This year"
	sectionForFun    = "Just for fun"
)

var defaultInterviewQuestions = []defaultInterviewQuestion{
	{"fav-color", sectionFavorites, "What's your favorite color?", 0, 0},
	{"fav-food", sectionFavorites, "What's your favorite food?", 0, 0},
	{"fav-snack", sectionFavorites, "What's your favorite snack?", 0, 0},
	{"fav-treat", sectionFavorites, "What's your favorite dessert or treat?", 0, 0},
	{"fav-breakfast", sectionFavorites, "What's your favorite breakfast?", 0, 0},
	{"fav-drink", sectionFavorites, "What's your favorite drink?", 0, 0},
	{"fav-animal", sectionFavorites, "What's your favorite animal?", 0, 0},
	{"fav-show", sectionFavorites, "What's your favorite TV show?", 0, 0},
	{"fav-movie", sectionFavorites, "What's your favorite movie?", 0, 0},
	{"fav-book", sectionFavorites, "What's your favorite book?", 0, 0},
	{"fav-song", sectionFavorites, "What's your favorite song or singer?", 0, 0},
	{"fav-toy", sectionFavorites, "What's your favorite toy?", 0, 0},
	{"fav-game", sectionFavorites, "What's your favorite game?", 0, 0},
	{"fav-sport", sectionFavorites, "What's your favorite sport?", 0, 0},
	{"fav-outdoors", sectionFavorites, "What's your favorite thing to do outside?", 0, 0},
	{"fav-rainy-day", sectionFavorites, "What's your favorite thing to do on a rainy day?", 0, 0},
	{"fav-place", sectionFavorites, "What's your favorite place to go?", 0, 0},
	{"fav-holiday", sectionFavorites, "What's your favorite holiday?", 0, 0},
	{"fav-season", sectionFavorites, "What's your favorite season?", 0, 0},
	{"fav-subject", sectionFavorites, "What's your favorite subject at school?", 5, 0},
	{"fav-app", sectionFavorites, "What's your favorite video game or app?", 6, 0},

	{"good-at", sectionAboutMe, "What are you really good at?", 0, 0},
	{"learning", sectionAboutMe, "What are you learning right now?", 0, 0},
	{"happy", sectionAboutMe, "What makes you happy?", 0, 0},
	{"sad", sectionAboutMe, "What makes you sad?", 0, 0},
	{"scared", sectionAboutMe, "What scares you?", 0, 0},
	{"laugh", sectionAboutMe, "What makes you laugh?", 0, 0},
	{"dislike-food", sectionAboutMe, "What's a food you don't like?", 0, 0},
	{"grow-up", sectionAboutMe, "What do you want to be when you grow up?", 0, 0},
	{"best-age", sectionAboutMe, "What's the best thing about being {age}?", 0, 0},

	{"best-friend", sectionPeople, "Who is your best friend?", 0, 0},
	{"teacher", sectionPeople, "Who is your teacher?", 4, 0},
	{"family-fun", sectionPeople, "What do you love doing with your family?", 0, 0},
	{"funniest", sectionPeople, "Who is the funniest person you know?", 0, 0},
	{"parent-says", sectionPeople, "What does your mom or dad always say?", 0, 0},

	{"year-best", sectionThisYear, "What was the best part of this year?", 0, 0},
	{"year-hard", sectionThisYear, "What was something hard you did?", 0, 0},
	{"year-next", sectionThisYear, "What are you excited about next year?", 0, 0},
	{"year-present", sectionThisYear, "What was the best present you got?", 0, 0},

	{"superpower", sectionForFun, "If you could have one superpower, what would it be?", 0, 0},
	{"be-animal", sectionForFun, "If you could be any animal, what would you be?", 0, 0},
	{"one-wish", sectionForFun, "If you had one wish, what would it be?", 0, 0},
	{"million", sectionForFun, "What would you buy with a million dollars?", 0, 0},
	{"eat-forever", sectionForFun, "What would you eat every day forever?", 0, 0},
	{"travel", sectionForFun, "Where in the world would you go?", 0, 0},
	{"fav-word", sectionForFun, "What's your favorite word?", 0, 0},
}

func defaultInterviewQuestionByKey(key string) (defaultInterviewQuestion, bool) {
	for _, q := range defaultInterviewQuestions {
		if q.Key == key {
			return q, true
		}
	}
	return defaultInterviewQuestion{}, false
}

func (d defaultInterviewQuestion) row(familyId int) InterviewQuestion {
	return InterviewQuestion{
		FamilyId:    familyId,
		Text:        d.Text,
		Section:     d.Section,
		MinAgeYears: d.MinAge,
		MaxAgeYears: d.MaxAge,
		DefaultKey:  d.Key,
	}
}
