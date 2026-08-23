//go:build release

package cfg

const IsRelease = true
const DBPath = "/srv/apps/family/shared/data/db.bolt"
const StaticDir = "/srv/apps/family/shared/static/"

// LogDir lives under shared/ rather than in the release directory. The service's
// WorkingDirectory is /srv/apps/family/current, so a relative "logs" put the log
// file *inside* the release, where every deploy started an empty one and the
// sixth deploy after an incident deleted the evidence.
const LogDir = "/srv/apps/family/shared/logs"
const SiteURL = "https://familyrecord.app"
const EnableFaceTagging = true
const FaceModelsDir = "/srv/apps/family/shared/models"
const FaceAnalysisSocket = "/run/family-face/face.sock"
