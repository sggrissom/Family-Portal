package backend

import (
	"strings"

	"go.hasen.dev/vbolt"
)

// recorderName names the account that added a record, for members of the
// family the record belongs to. Linked families see records, not who in
// another household typed them.
func recorderName(tx *vbolt.Tx, viewer User, familyId int, createdBy int) string {
	if createdBy == 0 {
		return ""
	}
	if _, ok := FindMembership(tx, viewer.Id, familyId); !ok {
		return ""
	}
	return strings.TrimSpace(GetUser(tx, createdBy).Name)
}

func exportRecorders(tx *vbolt.Tx) func(userId int) string {
	names := map[int]string{}
	return func(userId int) string {
		if userId == 0 {
			return ""
		}
		name, ok := names[userId]
		if !ok {
			name = strings.TrimSpace(GetUser(tx, userId).Name)
			names[userId] = name
		}
		return name
	}
}

// importRecorders maps an exported recorder name back to a member of the
// importing family. A name two members share stays unattributed.
func importRecorders(tx *vbolt.Tx, familyId int) func(name string) int {
	ids := map[string]int{}
	for _, membership := range GetFamilyMemberships(tx, familyId) {
		name := strings.TrimSpace(GetUser(tx, membership.UserId).Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, taken := ids[key]; taken {
			ids[key] = 0
		} else {
			ids[key] = membership.UserId
		}
	}
	return func(name string) int {
		if name = strings.TrimSpace(name); name == "" {
			return 0
		}
		return ids[strings.ToLower(name)]
	}
}
