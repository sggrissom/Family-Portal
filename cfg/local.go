//go:build !release

package cfg

const IsRelease = false
const DBPath = ".serve/db.bolt"
const StaticDir = ".serve/static/"
const SiteURL = "http://localhost:8666"
