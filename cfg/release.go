//go:build release

package cfg

const IsRelease = true
const DBPath = "/srv/apps/family/shared/data/db.bolt"
const StaticDir = "/srv/apps/family/shared/static/"
const SiteURL = "https://grissom.zone"
const EnableFaceTagging = true
const FaceModelsDir = "/srv/apps/family/shared/models"
const FaceAnalysisSocket = "/run/family-face/face.sock"
