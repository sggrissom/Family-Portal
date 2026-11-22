package backend

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterRecapLinkMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CreateRecapLink)
	vbeam.RegisterProc(app, ListRecapLinks)
	vbeam.RegisterProc(app, RevokeRecapLink)
	vbeam.RegisterProc(app, GetRecap)
}

var recapResultKinds = map[string]bool{
	ResultKindAdjudication: true,
	ResultKindPlacement:    true,
	ResultKindAward:        true,
	ResultKindScore:        true,
}

func recapText(value string) string {
	value = strings.TrimSpace(value)
	if runes := []rune(value); len(runes) > maxRecapTextLength {
		value = strings.TrimSpace(string(runes[:maxRecapTextLength]))
	}
	return value
}

func buildPublishedRecap(tx *vbolt.Tx, event Event, in RecapInput) (PublishedRecap, error) {
	if len(in.Routines) == 0 {
		return PublishedRecap{}, ErrRecapEmpty
	}
	if len(in.Routines) > maxRecapRoutines {
		return PublishedRecap{}, ErrRecapTooLarge
	}

	recap := PublishedRecap{
		EventName:     recapText(in.EventName),
		Subtitle:      recapText(in.Subtitle),
		Place:         recapText(in.Place),
		SnapshotLabel: recapText(in.SnapshotLabel),
	}
	if recap.EventName == "" {
		recap.EventName = event.Name
	}

	seenEntries := make(map[int]bool)
	seenAppearances := make(map[int]bool)
	for _, routineIn := range in.Routines {
		entry := GetEntryById(tx, routineIn.EntryId)
		if entry.Id == 0 || entry.FamilyId != event.FamilyId || entry.SeasonId != event.SeasonId || seenEntries[entry.Id] {
			return PublishedRecap{}, ErrRecapNotThisEvent
		}
		seenEntries[entry.Id] = true
		if len(routineIn.Performances) == 0 {
			return PublishedRecap{}, ErrRecapEmpty
		}
		if len(routineIn.Performances) > maxResultsPerAppearance {
			return PublishedRecap{}, ErrRecapTooLarge
		}

		routine := PublishedRoutine{
			Name:        recapText(routineIn.Name),
			Descriptors: recapText(routineIn.Descriptors),
			Roster:      recapText(routineIn.Roster),
			Summary:     recapText(routineIn.Summary),
		}
		if routine.Name == "" {
			routine.Name = entry.Name
		}
		for _, perfIn := range routineIn.Performances {
			appearance := GetAppearanceById(tx, perfIn.AppearanceId)
			if appearance.Id == 0 || appearance.EventId != event.Id || appearance.EntryId != entry.Id || seenAppearances[appearance.Id] {
				return PublishedRecap{}, ErrRecapNotThisEvent
			}
			seenAppearances[appearance.Id] = true
			if appearance.Revision != perfIn.Revision {
				return PublishedRecap{}, ErrRecapOutOfDate
			}
			if len(perfIn.Results) > maxResultsPerAppearance {
				return PublishedRecap{}, ErrRecapTooLarge
			}

			performance := PublishedPerformance{
				When:    recapText(perfIn.When),
				Matchup: recapText(perfIn.Matchup),
			}
			for _, resultIn := range perfIn.Results {
				if !recapResultKinds[resultIn.Kind] {
					return PublishedRecap{}, ErrInvalidResultKind
				}
				performance.Results = append(performance.Results, PublishedResult{
					Kind:   resultIn.Kind,
					Text:   recapText(resultIn.Text),
					Detail: recapText(resultIn.Detail),
				})
			}
			routine.Performances = append(routine.Performances, performance)
		}
		recap.Routines = append(recap.Routines, routine)
	}
	return recap, nil
}

func canRevokeRecapLink(tx *vbolt.Tx, user User, link RecapLink) bool {
	if CanAccessFamily(tx, user, link.FamilyId, AccessAdmin) {
		return true
	}
	return link.CreatedBy == user.Id && CanAccessFamily(tx, user, link.FamilyId, AccessContribute)
}

func recapLinkView(tx *vbolt.Tx, user User, link RecapLink, now time.Time) RecapLinkView {
	view := RecapLinkView{
		Id:            link.Id,
		CreatedAt:     link.CreatedAt,
		Expired:       !link.liveAt(now),
		CanRevoke:     canRevokeRecapLink(tx, user, link),
		RoutineCount:  len(link.Recap.Routines),
		SnapshotLabel: link.Recap.SnapshotLabel,
	}
	if !link.ExpiresAt.IsZero() {
		expires := link.ExpiresAt
		view.ExpiresAt = &expires
	}
	if creator := GetUser(tx, link.CreatedBy); creator.Id != 0 {
		view.CreatedByName = creator.Name
	}
	return view
}

func CreateRecapLink(ctx *vbeam.Context, req CreateRecapLinkRequest) (resp CreateRecapLinkResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	event, err := getEventForUser(ctx.Tx, req.EventId, user, AccessContribute)
	if err != nil {
		return
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > maxRecapExpiryDays {
		err = ErrRecapExpiry
		return
	}
	if len(getEventRecapLinks(ctx.Tx, event.Id)) >= maxRecapLinksPerEvent {
		err = ErrRecapLinkLimit
		return
	}
	recap, err := buildPublishedRecap(ctx.Tx, event, req.Recap)
	if err != nil {
		return
	}

	token, err := generateToken(recapTokenBytes)
	if err != nil {
		return
	}

	now := time.Now()
	link := RecapLink{
		FamilyId:  event.FamilyId,
		EventId:   event.Id,
		CreatedBy: user.Id,
		TokenHash: hashRecapToken(token),
		CreatedAt: now,
		Recap:     recap,
	}
	if req.ExpiresInDays > 0 {
		link.ExpiresAt = now.AddDate(0, 0, req.ExpiresInDays)
	}

	vbeam.UseWriteTx(ctx)
	link.Id = vbolt.NextIntId(ctx.Tx, RecapLinkBkt)
	writeRecapLinkTx(ctx.Tx, &link)
	resp.Link = recapLinkView(ctx.Tx, user, link, now)
	resp.URL = recapURL(token)
	vbolt.TxCommit(ctx.Tx)
	return
}

func ListRecapLinks(ctx *vbeam.Context, req ListRecapLinksRequest) (resp ListRecapLinksResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	event, err := getEventForUser(ctx.Tx, req.EventId, user, AccessContribute)
	if err != nil {
		return
	}

	now := time.Now()
	links := getEventRecapLinks(ctx.Tx, event.Id)
	sort.Slice(links, func(i, j int) bool { return links[i].Id > links[j].Id })
	resp.Links = make([]RecapLinkView, 0, len(links))
	for _, link := range links {
		resp.Links = append(resp.Links, recapLinkView(ctx.Tx, user, link, now))
	}
	return
}

func RevokeRecapLink(ctx *vbeam.Context, req RevokeRecapLinkRequest) (resp DeleteResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	link := GetRecapLinkById(ctx.Tx, req.Id)
	if link.Id == 0 || !CanAccessFamily(ctx.Tx, user, link.FamilyId, AccessContribute) {
		err = ErrRecapLinkNotFound
		return
	}
	if !canRevokeRecapLink(ctx.Tx, user, link) {
		err = ErrRecapLinkNotAllowed
		return
	}

	vbeam.UseWriteTx(ctx)
	deleteRecapLinkTx(ctx.Tx, link)
	vbolt.TxCommit(ctx.Tx)
	resp.Success = true
	return
}

// GetRecap is the public read behind /recap/<token>: no sign-in, and the
// same refusal for unknown, expired and revoked tokens.
func GetRecap(ctx *vbeam.Context, req GetRecapRequest) (resp GetRecapResponse, err error) {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		err = ErrRecapUnavailable
		return
	}
	link, found := findRecapLinkByToken(ctx.Tx, token)
	if !found || !link.liveAt(time.Now()) {
		err = ErrRecapUnavailable
		return
	}
	resp.Recap = withEmptyLists(link.Recap)
	return
}

// withEmptyLists sends [] rather than null for lists the snapshot stored empty.
func withEmptyLists(recap PublishedRecap) PublishedRecap {
	routines := make([]PublishedRoutine, len(recap.Routines))
	for i, routine := range recap.Routines {
		performances := make([]PublishedPerformance, len(routine.Performances))
		for j, performance := range routine.Performances {
			if performance.Results == nil {
				performance.Results = []PublishedResult{}
			}
			performances[j] = performance
		}
		routine.Performances = performances
		routines[i] = routine
	}
	recap.Routines = routines
	return recap
}

// redactRecapPath keeps share tokens out of log lines.
func redactRecapPath(path string) string {
	if !strings.HasPrefix(path, RecapPathPrefix) {
		return path
	}
	_, rest, _ := strings.Cut(strings.TrimPrefix(path, RecapPathPrefix), "/")
	if rest == "" {
		return RecapPathPrefix + "[redacted]"
	}
	return RecapPathPrefix + "[redacted]/" + rest
}

// addRecapPageHeaders covers the app shell served at /recap/<token>. The shell
// holds no recap; GetRecap checks the link on every load.
func addRecapPageHeaders(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, RecapPathPrefix) {
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
}
