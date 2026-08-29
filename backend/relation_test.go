package backend

import (
	"family/cfg"
	"os"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

type relationFixture struct {
	db     *vbolt.DB
	user   User
	family int

	me      Person
	partner Person
	kid     Person
	kid2    Person
	sister  Person
	niece   Person
	grandma Person
}

func setupRelationFixture(t *testing.T) (relationFixture, func()) {
	t.Helper()

	testDBPath := "test_relation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	previousDb := appDb
	appDb = db
	cleanup := func() {
		appDb = previousDb
		db.Close()
		os.Remove(testDBPath)
	}

	fx := relationFixture{db: db}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		fx.user = AddUserTx(tx, CreateAccountRequest{Name: "Me", Email: "rel@example.com"}, hash)
		fx.family = fx.user.FamilyId

		add := func(name string, gender GenderType, birthdate string) Person {
			person, err := AddPersonTx(tx, AddPersonRequest{
				Name: name, Gender: int(gender), Birthdate: birthdate,
			}, fx.family)
			if err != nil {
				t.Fatalf("AddPersonTx %s: %v", name, err)
			}
			return person
		}

		fx.me = add("Me", Female, "1986-02-02")
		fx.partner = add("Sam", Male, "1984-03-03")
		fx.kid = add("Mia", Female, "2016-04-11")
		fx.kid2 = add("Ben", Male, "2019-08-01")
		fx.sister = add("Kate", Female, "1988-07-07")
		fx.niece = add("Ada", Female, "2018-01-01")
		fx.grandma = add("Rose", Female, "1958-05-05")

		fx.user.PersonId = fx.me.Id
		vbolt.Write(tx, UsersBkt, fx.user.Id, &fx.user)

		edges := []Relation{
			{FromId: fx.me.Id, ToId: fx.kid.Id, Kind: RelationParent},
			{FromId: fx.me.Id, ToId: fx.kid2.Id, Kind: RelationParent},
			{FromId: fx.me.Id, ToId: fx.partner.Id, Kind: RelationPartner},
			{FromId: fx.me.Id, ToId: fx.sister.Id, Kind: RelationSibling},
			{FromId: fx.sister.Id, ToId: fx.niece.Id, Kind: RelationParent},
			{FromId: fx.grandma.Id, ToId: fx.me.Id, Kind: RelationParent},
		}
		for _, edge := range edges {
			if _, err := AddRelationTx(tx, edge); err != nil {
				t.Fatalf("AddRelationTx: %v", err)
			}
		}

		vbolt.TxCommit(tx)
	})

	return fx, cleanup
}

func TestRelationLabelsWalkTheGraph(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		cases := []struct {
			subject Person
			target  Person
			want    string
		}{
			{fx.me, fx.kid, "daughter"},
			{fx.me, fx.kid2, "son"},
			{fx.kid, fx.me, "mother"},
			{fx.me, fx.partner, "husband"},
			{fx.partner, fx.me, "wife"},
			{fx.me, fx.sister, "sister"},
			{fx.me, fx.niece, "niece"},
			{fx.niece, fx.me, "aunt"},
			{fx.me, fx.grandma, "mother"},
			{fx.kid, fx.grandma, "grandmother"},
			{fx.grandma, fx.kid, "granddaughter"},
			{fx.kid, fx.niece, "cousin"},
			{fx.kid, fx.kid2, "brother"},
		}
		for _, tc := range cases {
			got := RelationLabel(tx, tc.subject, tc.target)
			if got != tc.want {
				t.Errorf("%s -> %s = %q, want %q", tc.subject.Name, tc.target.Name, got, tc.want)
			}
		}
	})
}

func TestSiblingsComeFromASharedParent(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		siblings := SiblingsOf(tx, fx.kid.Id)
		if !containsId(siblings, fx.kid2.Id) {
			t.Errorf("Mia's siblings = %v, want Ben (%d) via the shared parent", siblings, fx.kid2.Id)
		}
		if containsId(siblings, fx.kid.Id) {
			t.Error("SiblingsOf included the person themselves")
		}
		if containsId(siblings, fx.niece.Id) {
			t.Error("a cousin was counted as a sibling")
		}
	})
}

func TestUnrelatedPeopleGetNoLabel(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		stranger, err := AddPersonTx(tx, AddPersonRequest{
			Name: "Nobody", Gender: int(Unknown), Birthdate: "1990-01-01",
		}, fx.family)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		if got := RelationLabel(tx, fx.me, stranger); got != "" {
			t.Errorf("unrelated person labelled %q, want no label", got)
		}
		if got := RelationLabel(tx, fx.me, fx.me); got != "" {
			t.Errorf("self labelled %q, want no label", got)
		}
		vbolt.TxCommit(tx)
	})
}

func TestStatedRelationCarriesDirection(t *testing.T) {
	me, anchor := 10, 20
	cases := []struct {
		stated StatedRelation
		want   Relation
	}{
		{StatedChild, Relation{FromId: anchor, ToId: me, Kind: RelationParent}},
		{StatedParent, Relation{FromId: me, ToId: anchor, Kind: RelationParent}},
		{StatedSibling, Relation{FromId: me, ToId: anchor, Kind: RelationSibling}},
		{StatedPartner, Relation{FromId: me, ToId: anchor, Kind: RelationPartner}},
	}
	for _, tc := range cases {
		got, ok := tc.stated.edge(me, anchor)
		if !ok || got != tc.want {
			t.Errorf("stated %d edge = %+v (ok=%v), want %+v", tc.stated, got, ok, tc.want)
		}
	}
	if _, ok := StatedNone.edge(me, anchor); ok {
		t.Error("StatedNone produced an edge")
	}
}

func TestSymmetricEdgesAreNotDuplicated(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	before := len(relationRows(t, fx.db))

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		// The fixture already states me->sister; the reverse says the same thing.
		if _, err := AddRelationTx(tx, Relation{
			FromId: fx.sister.Id, ToId: fx.me.Id, Kind: RelationSibling,
		}); err != nil {
			t.Fatalf("AddRelationTx: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	if after := len(relationRows(t, fx.db)); after != before {
		t.Errorf("restating a symmetric relation added a row: %d -> %d", before, after)
	}
}

func TestRelationToSelfIsRejected(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		_, err := AddRelationTx(tx, Relation{
			FromId: fx.me.Id, ToId: fx.me.Id, Kind: RelationSibling,
		})
		if err != ErrRelationToSelf {
			t.Errorf("self-relation returned %v, want ErrRelationToSelf", err)
		}
	})
}

func TestDeletingAPersonDropsTheirEdges(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		deletePersonRelationsTx(tx, fx.sister.Id)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if rows := GetPersonRelationsTx(tx, fx.sister.Id); len(rows) != 0 {
			t.Errorf("deleted person kept %d edges", len(rows))
		}
		if got := RelationLabel(tx, fx.me, fx.niece); got != "" {
			t.Errorf("niece still labelled %q after the connecting person went away", got)
		}
		if got := RelationLabel(tx, fx.me, fx.kid); got != "daughter" {
			t.Errorf("unrelated edge was disturbed: %q", got)
		}
	})
}

func TestMergeMovesEdgesToTheSurvivor(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	var duplicate Person
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		var err error
		duplicate, err = AddPersonTx(tx, AddPersonRequest{
			Name: "Mia (dup)", Gender: int(Female), Birthdate: "2016-04-11",
		}, fx.family)
		if err != nil {
			t.Fatalf("AddPersonTx: %v", err)
		}
		if _, err := AddRelationTx(tx, Relation{
			FromId: fx.grandma.Id, ToId: duplicate.Id, Kind: RelationParent,
		}); err != nil {
			t.Fatalf("AddRelationTx: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		movePersonRelationsTx(tx, duplicate.Id, fx.kid.Id)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if rows := GetPersonRelationsTx(tx, duplicate.Id); len(rows) != 0 {
			t.Errorf("merged-away person kept %d edges", len(rows))
		}
		if !containsId(parentsOf(tx, fx.kid.Id), fx.grandma.Id) {
			t.Error("the survivor did not inherit the merged person's parent edge")
		}
	})
}

// The fixture deliberately leaves Sam off the kids' parent edges, which is the
// step-family shape: he is married to Me but is nobody's parent.
func TestStepAndInLawLabels(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		cases := []struct {
			subject Person
			target  Person
			want    string
		}{
			{fx.partner, fx.kid, "stepdaughter"},
			{fx.partner, fx.kid2, "stepson"},
			{fx.kid, fx.partner, "stepfather"},
			{fx.partner, fx.grandma, "mother-in-law"},
			{fx.grandma, fx.partner, "son-in-law"},
			{fx.partner, fx.sister, "sister-in-law"},
			{fx.sister, fx.partner, "brother-in-law"},
		}
		for _, tc := range cases {
			got := RelationLabel(tx, tc.subject, tc.target)
			if got != tc.want {
				t.Errorf("%s -> %s = %q, want %q", tc.subject.Name, tc.target.Name, got, tc.want)
			}
		}
	})
}

func TestBloodRelationsBeatStepOnes(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		if _, err := AddRelationTx(tx, Relation{
			FromId: fx.partner.Id, ToId: fx.kid.Id, Kind: RelationParent,
		}); err != nil {
			t.Fatalf("AddRelationTx: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		if got := RelationLabel(tx, fx.partner, fx.kid); got != "daughter" {
			t.Errorf("Sam -> Mia = %q, want daughter once the parent edge exists", got)
		}
		if got := RelationLabel(tx, fx.partner, fx.kid2); got != "stepson" {
			t.Errorf("Sam -> Ben = %q, want stepson; only Mia got the parent edge", got)
		}
	})
}

func TestPersonRelationsListsStoredAndImplied(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		resp := personRelations(tx, fx.user, fx.kid)

		byName := make(map[string]RelationView)
		for _, row := range resp.Relations {
			byName[row.PersonName] = row
		}

		// Mia's only stored edge is the parent edge from Me.
		if row, ok := byName["Me"]; !ok || !row.Stored || row.Id == 0 {
			t.Errorf("Me = %+v, want a stored, removable row", row)
		}
		// Everything else follows from the graph and must not look removable.
		for _, name := range []string{"Ben", "Kate", "Rose", "Ada", "Sam"} {
			row, ok := byName[name]
			if !ok {
				t.Errorf("%s is missing; implied relationships should be listed", name)
				continue
			}
			if row.Stored || row.Id != 0 {
				t.Errorf("%s = %+v, want an implied row with no id", name, row)
			}
			if row.Label == "" {
				t.Errorf("%s was listed with no label", name)
			}
		}
	})
}

func TestOneCallStatesTheSameRelationAgainstSeveralPeople(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		err := applyStatedRelationTx(tx, fx.user, fx.partner.Id, StatedParent,
			[]int{fx.kid.Id, fx.kid2.Id, fx.kid.Id, 0, fx.partner.Id})
		if err != nil {
			t.Fatalf("applyStatedRelationTx: %v", err)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) {
		children := childrenOf(tx, fx.partner.Id)
		if len(children) != 2 {
			t.Errorf("Sam's children = %v, want exactly Mia and Ben", children)
		}
		for _, want := range []int{fx.kid.Id, fx.kid2.Id} {
			if !containsId(children, want) {
				t.Errorf("Sam's children = %v, missing %d", children, want)
			}
		}
	})
}

func relationRows(t *testing.T, db *vbolt.DB) (rows []Relation) {
	t.Helper()
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, RelationBkt, func(key int, row Relation) bool {
			rows = append(rows, row)
			return true
		})
	})
	return
}

func TestUpdatePersonAnswersWithTheRelationship(t *testing.T) {
	fx, cleanup := setupRelationFixture(t)
	defer cleanup()
	jwtKey = []byte("relation-test-secret-key-at-least-32")

	token, err := generateJwtTokenString(fx.user)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, procErr := UpdatePerson(&vbeam.Context{Tx: tx, Token: token}, UpdatePersonRequest{
			Id:        fx.kid.Id,
			Name:      "Mia Rose",
			Gender:    int(Female),
			Birthdate: "2016-04-11",
		})
		if procErr != nil {
			t.Fatalf("UpdatePerson() error = %v", procErr)
		}
		// An edit says nothing about relationships, so a response that omitted the
		// label would read as "no longer related" to a client that applies it.
		if resp.Person.Relationship != "daughter" {
			t.Errorf("relationship = %q, want \"daughter\"", resp.Person.Relationship)
		}
	})
}
