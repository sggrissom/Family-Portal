package backend

import (
	"sort"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminDigestMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetWeeklyDigest)
}

const digestWindow = 7 * 24 * time.Hour

type DigestPerson struct {
	Name      string    `json:"name"`
	SignedIn  bool      `json:"signedIn"`
	LastLogin time.Time `json:"lastLogin"`
	Joined    bool      `json:"joined"`
	Photos    int       `json:"photos"`
	Messages  int       `json:"messages"`
}

type WeeklyDigestResponse struct {
	Since        time.Time      `json:"since"`
	WindowDays   int            `json:"windowDays"`
	Photos       int            `json:"photos"`
	Milestones   int            `json:"milestones"`
	Measurements int            `json:"measurements"`
	Messages     int            `json:"messages"`
	People       []DigestPerson `json:"people"`
	Accounts     int            `json:"accounts"`
	Absent       int            `json:"absent"`
	Quiet        bool           `json:"quiet"`
}

func GetWeeklyDigest(ctx *vbeam.Context, req Empty) (resp WeeklyDigestResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	since := time.Now().Add(-digestWindow)
	resp.Since = since
	resp.WindowDays = int(digestWindow / (24 * time.Hour))
	resp.People = []DigestPerson{}

	contributors := make(map[int]*DigestPerson)
	contributor := func(userId int) *DigestPerson {
		entry := contributors[userId]
		if entry == nil {
			entry = &DigestPerson{}
			contributors[userId] = entry
		}
		return entry
	}

	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if image.CreatedAt.After(since) {
			resp.Photos++
			contributor(image.OwnerUserId).Photos++
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, ChatMessagesBkt, func(key int, message ChatMessage) bool {
		if message.CreatedAt.After(since) {
			resp.Messages++
			contributor(message.UserId).Messages++
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, MilestoneBkt, func(key int, milestone Milestone) bool {
		if milestone.CreatedAt.After(since) {
			resp.Milestones++
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, GrowthDataBkt, func(key int, growth GrowthData) bool {
		if growth.CreatedAt.After(since) {
			resp.Measurements++
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		resp.Accounts++

		entry := contributors[user.Id]
		signedIn := user.LastLogin.After(since)
		joined := user.Creation.After(since)
		if entry == nil {
			if !signedIn && !joined {
				resp.Absent++
				return true
			}
			entry = contributor(user.Id)
		}

		entry.Name = user.Name
		entry.SignedIn = signedIn
		entry.LastLogin = user.LastLogin
		entry.Joined = joined
		return true
	})

	for _, entry := range contributors {
		if entry.Name == "" {
			entry.Name = "a deleted account"
		}
		resp.People = append(resp.People, *entry)
	}

	sort.Slice(resp.People, func(i, j int) bool {
		a, b := resp.People[i], resp.People[j]
		if a.Photos+a.Messages != b.Photos+b.Messages {
			return a.Photos+a.Messages > b.Photos+b.Messages
		}
		if !a.LastLogin.Equal(b.LastLogin) {
			return a.LastLogin.After(b.LastLogin)
		}
		return a.Name < b.Name
	})

	resp.Quiet = resp.Photos == 0 &&
		resp.Milestones == 0 &&
		resp.Measurements == 0 &&
		resp.Messages == 0 &&
		len(resp.People) == 0

	return
}
