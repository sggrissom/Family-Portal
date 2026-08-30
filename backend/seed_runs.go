package backend

import (
	"family/cfg"
	"sort"
	"time"

	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

// A SeedRun is the receipt for one call to SeedDemoData: exactly which accounts
// and families it created. Removal works from this list and nothing else, so a
// cleanup can never reach a record the seeder did not write.
type SeedRun struct {
	Id        int       `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy int       `json:"createdBy"`
	Domain    string    `json:"domain"`
	UserIds   []int     `json:"userIds"`
	Emails    []string  `json:"emails"`
	FamilyIds []int     `json:"familyIds"`
}

func PackSeedRun(self *SeedRun, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Int(&self.CreatedBy, buf)
	vpack.String(&self.Domain, buf)
	vpack.Slice(&self.UserIds, vpack.Int, buf)
	vpack.Slice(&self.Emails, vpack.String, buf)
	vpack.Slice(&self.FamilyIds, vpack.Int, buf)
}

var SeedRunsBkt = vbolt.Bucket(&cfg.Info, "seed_runs", vpack.FInt, PackSeedRun)

func writeSeedRunTx(tx *vbolt.Tx, run *SeedRun) {
	vbolt.Write(tx, SeedRunsBkt, run.Id, run)
}

func GetSeedRun(tx *vbolt.Tx, runId int) (run SeedRun) {
	vbolt.Read(tx, SeedRunsBkt, runId, &run)
	return
}

func ListSeedRunsTx(tx *vbolt.Tx) (runs []SeedRun) {
	vbolt.IterateAll(tx, SeedRunsBkt, func(key int, run SeedRun) bool {
		runs = append(runs, run)
		return true
	})
	sort.Slice(runs, func(i, j int) bool { return runs[i].Id > runs[j].Id })
	return
}

type SeedRemoval struct {
	RemovedEmails     []string
	SkippedEmails     []string
	DestroyedFamilies []int
	SurvivingFamilies []int
	OrphanedPhotos    []Image
}

// RemoveSeedRunTx deletes the accounts a run created and nothing else. A user
// whose record no longer matches the run — a reused id, a changed address, the
// site administrator — is skipped rather than deleted. Families the seeded
// accounts share with anyone else survive, because deleteAccountTx only
// destroys a family once its last member leaves.
func RemoveSeedRunTx(tx *vbolt.Tx, run SeedRun) (removal SeedRemoval, err error) {
	if run.Id == 0 {
		err = ErrSeedRunNotFound
		return
	}

	owned := make(map[string]bool, len(run.Emails))
	for _, email := range run.Emails {
		owned[email] = true
	}

	for _, userId := range run.UserIds {
		user := GetUser(tx, userId)
		if user.Id == 0 {
			continue
		}
		if user.Id == AdminUserId || !owned[user.Email] || GetUserId(tx, user.Email) != user.Id {
			removal.SkippedEmails = append(removal.SkippedEmails, user.Email)
			continue
		}
		photos, destroyed := deleteAccountTx(tx, user)
		removal.OrphanedPhotos = append(removal.OrphanedPhotos, photos...)
		removal.DestroyedFamilies = append(removal.DestroyedFamilies, destroyed...)
		removal.RemovedEmails = append(removal.RemovedEmails, user.Email)
	}

	for _, familyId := range run.FamilyIds {
		if GetFamily(tx, familyId).Id != 0 {
			removal.SurvivingFamilies = append(removal.SurvivingFamilies, familyId)
		}
	}

	vbolt.Delete(tx, SeedRunsBkt, run.Id)
	return
}
