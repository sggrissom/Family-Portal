//go:build release

package cfg

const IsRelease = true
const DBPath = "/srv/apps/family/shared/data/db.bolt"
const StaticDir = "/srv/apps/family/shared/static/"

const LogDir = "/srv/apps/family/shared/logs"
const SiteURL = "https://familyrecord.app"
const EnableFaceTagging = true
const FaceModelsDir = "/srv/apps/family/shared/models"
const FaceAnalysisSocket = "/run/family-face/face.sock"

const FamilyStorageQuotaBytes = 10 << 30
const MinFreeDiskBytes = 1 << 30
