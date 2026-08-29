//go:build !release

package cfg

const IsRelease = false
const DBPath = ".serve/db.bolt"
const StaticDir = ".serve/static/"
const LogDir = "logs"
const SiteURL = "http://localhost:8666"
const EnableFaceTagging = false
const FaceModelsDir = ""
const FaceAnalysisSocket = ""

const FamilyStorageQuotaBytes = 10 << 30
const MinFreeDiskBytes = 1 << 30
