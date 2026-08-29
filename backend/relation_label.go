package backend

import "go.hasen.dev/vbolt"

type genderedTerm struct {
	male    string
	female  string
	neutral string
}

func (t genderedTerm) forGender(gender GenderType) string {
	switch gender {
	case Male:
		return t.male
	case Female:
		return t.female
	}
	return t.neutral
}

var (
	termChild       = genderedTerm{"son", "daughter", "child"}
	termParent      = genderedTerm{"father", "mother", "parent"}
	termSibling     = genderedTerm{"brother", "sister", "sibling"}
	termPartner     = genderedTerm{"husband", "wife", "partner"}
	termGrandchild  = genderedTerm{"grandson", "granddaughter", "grandchild"}
	termGrandparent = genderedTerm{"grandfather", "grandmother", "grandparent"}
	termSiblingKid  = genderedTerm{"nephew", "niece", "nibling"}
	termParentSib   = genderedTerm{"uncle", "aunt", "aunt or uncle"}
	termCousin      = genderedTerm{"cousin", "cousin", "cousin"}
)

type relationMatch struct {
	term  genderedTerm
	group string
}

const (
	GroupParent   = "parent"
	GroupChild    = "child"
	GroupSibling  = "sibling"
	GroupPartner  = "partner"
	GroupExtended = "extended"
)

// RelationLabel names what target is to subject, walking at most two edges out.
// It returns "" when the two are not connected within that reach.
func RelationLabel(tx *vbolt.Tx, subject Person, target Person) string {
	match, found := relationBetween(tx, subject, target)
	if !found {
		return ""
	}
	return match.term.forGender(target.Gender)
}

// RelationGroup buckets the relationship for grouping in the UI.
func RelationGroup(tx *vbolt.Tx, subject Person, target Person) string {
	match, found := relationBetween(tx, subject, target)
	if !found {
		return ""
	}
	return match.group
}

func relationBetween(tx *vbolt.Tx, subject Person, target Person) (relationMatch, bool) {
	if subject.Id == 0 || target.Id == 0 || subject.Id == target.Id {
		return relationMatch{}, false
	}

	subjectParents := parentsOf(tx, subject.Id)
	subjectChildren := childrenOf(tx, subject.Id)

	if containsId(subjectChildren, target.Id) {
		return relationMatch{termChild, GroupChild}, true
	}
	if containsId(subjectParents, target.Id) {
		return relationMatch{termParent, GroupParent}, true
	}
	if containsId(statedPeers(tx, subject.Id, RelationPartner), target.Id) {
		return relationMatch{termPartner, GroupPartner}, true
	}
	if containsId(SiblingsOf(tx, subject.Id), target.Id) {
		return relationMatch{termSibling, GroupSibling}, true
	}

	for _, childId := range subjectChildren {
		if containsId(childrenOf(tx, childId), target.Id) {
			return relationMatch{termGrandchild, GroupExtended}, true
		}
	}
	for _, parentId := range subjectParents {
		if containsId(parentsOf(tx, parentId), target.Id) {
			return relationMatch{termGrandparent, GroupExtended}, true
		}
	}
	for _, siblingId := range SiblingsOf(tx, subject.Id) {
		if containsId(childrenOf(tx, siblingId), target.Id) {
			return relationMatch{termSiblingKid, GroupExtended}, true
		}
	}
	for _, parentId := range subjectParents {
		if containsId(SiblingsOf(tx, parentId), target.Id) {
			return relationMatch{termParentSib, GroupExtended}, true
		}
	}
	for _, parentId := range subjectParents {
		for _, auntId := range SiblingsOf(tx, parentId) {
			if containsId(childrenOf(tx, auntId), target.Id) {
				return relationMatch{termCousin, GroupExtended}, true
			}
		}
	}

	return relationMatch{}, false
}

// viewerPerson is the person record standing in for the acting user, used as
// the subject that derived relationship labels are phrased against.
func viewerPerson(tx *vbolt.Tx, user User) Person {
	if user.PersonId == 0 {
		return Person{}
	}
	return GetPersonById(tx, user.PersonId)
}

func labelPeopleFor(tx *vbolt.Tx, user User, people []Person) {
	subject := viewerPerson(tx, user)
	if subject.Id == 0 {
		return
	}
	for i := range people {
		people[i].Relationship = RelationLabel(tx, subject, people[i])
	}
}
