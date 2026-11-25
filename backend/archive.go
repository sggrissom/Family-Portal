package backend

// ArchiveClass says what the family archive does with a bucket: archives it,
// rebuilds it after import, or leaves it out. See ADR 0001.
type ArchiveClass int

const (
	BucketArchived ArchiveClass = iota + 1
	BucketDerived
	BucketExcluded
)

type archiveBucket struct {
	Class  ArchiveClass
	Reason string
}

func archived() archiveBucket { return archiveBucket{Class: BucketArchived} }

func derived(reason string) archiveBucket {
	return archiveBucket{Class: BucketDerived, Reason: reason}
}

func excluded(reason string) archiveBucket {
	return archiveBucket{Class: BucketExcluded, Reason: reason}
}

const (
	reasonAccount = "account state, not family history"
	reasonSharing = "sharing state, not family history"
	reasonSite    = "site operations, not family data"
)

// archiveBuckets classifies every vbolt bucket. A bucket missing from here
// fails TestEveryBucketIsClassifiedForTheArchive.
var archiveBuckets = map[string]archiveBucket{
	"families":                 archived(),
	"people":                   archived(),
	"growth_data":              archived(),
	"relation":                 archived(),
	"tags":                     archived(),
	"milestones":               archived(),
	"milestone_tags":           archived(),
	"milestone_photos":         archived(),
	"images":                   archived(),
	"photo_person":             archived(),
	"photo_tags":               archived(),
	"photo_faces":              archived(),
	"family_places":            archived(),
	"video_transcripts":        archived(),
	"activities":               archived(),
	"seasons":                  archived(),
	"activity_events":          archived(),
	"activity_entries":         archived(),
	"entry_members":            archived(),
	"appearances":              archived(),
	"appearance_photos":        archived(),
	"activity_results":         archived(),
	"activity_event_photos":    archived(),
	"interviews":               archived(),
	"interview_questions":      archived(),
	"interview_question_lists": archived(),
	"books":                    archived(),

	"milestone_embeddings": derived("text embeddings, recomputed from moments"),
	"photo_embeddings":     derived("image embeddings, recomputed from photos"),
	"photo_features":       derived("vision analysis output, recomputed from photos"),
	"speech_vectors":       derived("transcript embeddings, recomputed from transcripts"),
	"video_frames":         derived("frames sampled from the video file"),

	"users":                       excluded(reasonAccount),
	"passwd":                      excluded(reasonAccount),
	"email":                       excluded(reasonAccount),
	"email_verifications":         excluded(reasonAccount),
	"email_verifications_by_hash": excluded(reasonAccount),
	"password_resets":             excluded(reasonAccount),
	"password_resets_by_hash":     excluded(reasonAccount),
	"refresh_tokens":              excluded(reasonAccount),
	"refresh_tokens_by_token":     excluded(reasonAccount),
	"apple_refresh_tokens":        excluded(reasonAccount),
	"invite_codes":                excluded(reasonAccount),
	"notification_preferences":    excluded(reasonAccount),
	"push_device_tokens":          excluded(reasonAccount),
	"push_device_token_by_token":  excluded(reasonAccount),
	"family_membership":           excluded(reasonAccount),
	"chat_messages":               excluded("chat is conversation, not family records"),
	"family_link":                 excluded(reasonSharing),
	"person_family":               excluded(reasonSharing),
	"family_pages":                excluded(reasonSharing),
	"family_pages_by_slug":        excluded(reasonSharing),
	"recap_links":                 excluded(reasonSharing),
	"recap_links_by_hash":         excluded(reasonSharing),
	"tag_suggestions":             excluded("tag-suggestion state, regenerated as photos are analysed"),
	"suggest_state":               excluded("tag-suggestion state, regenerated as photos are analysed"),
	"family_suggest_gen":          excluded("tag-suggestion state, regenerated as photos are analysed"),
	"appearance_mutations":        excluded("appearance sync bookkeeping for offline clients"),
	"appearance_tombstones":       excluded("appearance sync bookkeeping for offline clients"),
	"video_uploads":               excluded("in-flight uploads, not finished media"),
	"archive_imports":             excluded("in-flight imports, not family data"),
	"media_deletions":             excluded(reasonSite),
	"log_review":                  excluded(reasonSite),
	"mobile_version":              excluded(reasonSite),
	"seed_runs":                   excluded(reasonSite),
}
