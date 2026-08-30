package backend

import (
	"strings"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

const seedPasswordMinLength = 8

func RegisterAdminSeedMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListSeedRuns)
	vbeam.RegisterProc(app, CreateSeedData)
	vbeam.RegisterProc(app, RemoveSeedData)
}

type SeedRunInfo struct {
	Id        int      `json:"id"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy int      `json:"createdBy"`
	Domain    string   `json:"domain"`
	Emails    []string `json:"emails"`
	Accounts  int      `json:"accounts"`
	Families  int      `json:"families"`
	Existing  int      `json:"existing"`
}

type ListSeedRunsResponse struct {
	Runs          []SeedRunInfo `json:"runs"`
	DefaultDomain string        `json:"defaultDomain"`
	MaxScale      int           `json:"maxScale"`
}

func ListSeedRuns(ctx *vbeam.Context, req Empty) (resp ListSeedRunsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	resp.DefaultDomain = DefaultSeedDomain
	resp.MaxScale = MaxSeedScale
	resp.Runs = []SeedRunInfo{}
	for _, run := range ListSeedRunsTx(ctx.Tx) {
		resp.Runs = append(resp.Runs, seedRunInfo(ctx.Tx, run))
	}
	return
}

func seedRunInfo(tx *vbolt.Tx, run SeedRun) SeedRunInfo {
	info := SeedRunInfo{
		Id:        run.Id,
		CreatedAt: run.CreatedAt.Format("2006-01-02 15:04"),
		CreatedBy: run.CreatedBy,
		Domain:    run.Domain,
		Emails:    run.Emails,
		Accounts:  len(run.UserIds),
	}
	if info.Emails == nil {
		info.Emails = []string{}
	}
	for _, familyId := range run.FamilyIds {
		if GetFamily(tx, familyId).Id != 0 {
			info.Families++
		}
	}
	for _, userId := range run.UserIds {
		if GetUser(tx, userId).Id != 0 {
			info.Existing++
		}
	}
	return info
}

type CreateSeedDataRequest struct {
	Password    string `json:"password"`
	EmailDomain string `json:"emailDomain"`
	Scale       int    `json:"scale"`
}

type SeedAccountInfo struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Family string `json:"family"`
	Access string `json:"access"`
}

type CreateSeedDataResponse struct {
	Run          SeedRunInfo       `json:"run"`
	Accounts     []SeedAccountInfo `json:"accounts"`
	People       int               `json:"people"`
	Milestones   int               `json:"milestones"`
	Measurements int               `json:"measurements"`
	ChatMessages int               `json:"chatMessages"`
}

// CreateSeedData writes the demo dataset into the live database. It only ever
// adds: a domain whose addresses are already taken is refused outright, so a
// run cannot take an existing account's login away from it.
func CreateSeedData(ctx *vbeam.Context, req CreateSeedDataRequest) (resp CreateSeedDataResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	if len(req.Password) < seedPasswordMinLength {
		err = ErrSeedPasswordRequired
		return
	}

	admin, _ := GetAuthUser(ctx)

	vbeam.UseWriteTx(ctx)
	summary, seedErr := SeedDemoData(ctx.Tx, SeedOptions{
		Password:    req.Password,
		EmailDomain: req.EmailDomain,
		Scale:       req.Scale,
		CreatedBy:   admin.Id,
	})
	if seedErr != nil {
		err = seedErr
		return
	}

	run := GetSeedRun(ctx.Tx, summary.RunId)
	resp.Run = seedRunInfo(ctx.Tx, run)
	resp.Accounts = make([]SeedAccountInfo, 0, len(summary.Accounts))
	for _, account := range summary.Accounts {
		resp.Accounts = append(resp.Accounts, SeedAccountInfo(account))
	}
	resp.People = summary.People
	resp.Milestones = summary.Milestones
	resp.Measurements = summary.Measurements
	resp.ChatMessages = summary.ChatMessages

	LogInfo(LogCategoryAdmin, "Seeded demo accounts", map[string]interface{}{
		"runId":    run.Id,
		"domain":   run.Domain,
		"accounts": len(run.UserIds),
		"adminId":  admin.Id,
	})

	vbolt.TxCommit(ctx.Tx)
	return
}

type RemoveSeedDataRequest struct {
	RunId        int    `json:"runId"`
	ConfirmValue string `json:"confirmValue"`
}

type RemoveSeedDataResponse struct {
	RemovedEmails     []string `json:"removedEmails"`
	SkippedEmails     []string `json:"skippedEmails"`
	DestroyedFamilies int      `json:"destroyedFamilies"`
	SurvivingFamilies int      `json:"survivingFamilies"`
	DeletedPhotos     int      `json:"deletedPhotos"`
}

// RemoveSeedData deletes the accounts one run created. Everything it touches
// comes from that run's own receipt, and the caller has to retype the domain.
func RemoveSeedData(ctx *vbeam.Context, req RemoveSeedDataRequest) (resp RemoveSeedDataResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	run := GetSeedRun(ctx.Tx, req.RunId)
	if run.Id == 0 {
		err = ErrSeedRunNotFound
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.ConfirmValue), run.Domain) {
		err = ErrSeedConfirmationMismatch
		return
	}

	vbeam.UseWriteTx(ctx)
	removal, removeErr := RemoveSeedRunTx(ctx.Tx, run)
	if removeErr != nil {
		err = removeErr
		return
	}

	resp.RemovedEmails = removal.RemovedEmails
	resp.SkippedEmails = removal.SkippedEmails
	if resp.RemovedEmails == nil {
		resp.RemovedEmails = []string{}
	}
	if resp.SkippedEmails == nil {
		resp.SkippedEmails = []string{}
	}
	resp.DestroyedFamilies = len(removal.DestroyedFamilies)
	resp.SurvivingFamilies = len(removal.SurvivingFamilies)
	resp.DeletedPhotos = len(removal.OrphanedPhotos)

	LogInfo(LogCategoryAdmin, "Removed seeded demo accounts", map[string]interface{}{
		"runId":   run.Id,
		"domain":  run.Domain,
		"removed": len(removal.RemovedEmails),
		"skipped": len(removal.SkippedEmails),
	})

	vbolt.TxCommit(ctx.Tx)

	for _, photo := range removal.OrphanedPhotos {
		if fileErr := deletePhotoFiles(photo); fileErr != nil {
			LogErrorSimple(LogCategoryPhoto, "Failed to delete photo files while removing seeded accounts", map[string]interface{}{
				"photoId": photo.Id,
				"error":   fileErr.Error(),
			})
		}
	}
	return
}
