package backend

import (
	"encoding/json"
	"errors"
	"family/cfg"
	"net/http"
	"strconv"
	"strings"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterMobileVersionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CheckMobileVersion)
	vbeam.RegisterProc(app, AdminSetMobileVersion)
	app.HandleFunc("GET /api/mobile-version", mobileVersionPolicyHandler(app.DB))
}

// Request/Response types

type CheckMobileVersionRequest struct {
	Platform   string `json:"platform"`   // "ios" or "android"
	AppVersion string `json:"appVersion"` // Current app version (semver)
}

type CheckMobileVersionResponse struct {
	Status         string `json:"status"` // "ok", "update_available", "update_required"
	MinimumVersion string `json:"minimumVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateUrl      string `json:"updateUrl"`
	UpdateMessage  string `json:"updateMessage"`
}

type AdminSetMobileVersionRequest struct {
	Platform       string `json:"platform"`       // "ios" or "android"
	MinimumVersion string `json:"minimumVersion"` // Below this, force update
	LatestVersion  string `json:"latestVersion"`  // Below this, suggest update
	UpdateUrl      string `json:"updateUrl"`      // App Store / Play Store URL
	UpdateMessage  string `json:"updateMessage"`  // Optional message to display
}

type AdminSetMobileVersionResponse struct {
	Success bool `json:"success"`
}

// Database types

type MobileVersionConfig struct {
	Id             int    // Platform key (1=ios, 2=android)
	Platform       string // "ios" or "android"
	MinimumVersion string // Semver — below this, force update
	LatestVersion  string // Semver — below this, suggest update
	UpdateUrl      string // App Store URL
	UpdateMessage  string // Optional message to display
}

func PackMobileVersionConfig(self *MobileVersionConfig, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.String(&self.Platform, buf)
	vpack.String(&self.MinimumVersion, buf)
	vpack.String(&self.LatestVersion, buf)
	vpack.String(&self.UpdateUrl, buf)
	vpack.String(&self.UpdateMessage, buf)
}

var MobileVersionBkt = vbolt.Bucket(&cfg.Info, "mobile_version", vpack.FInt, PackMobileVersionConfig)

func platformId(platform string) int {
	switch platform {
	case "ios":
		return 1
	case "android":
		return 2
	default:
		return 0
	}
}

// compareSemver compares two validated major.minor.patch version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareSemver(a, b string) int {
	aParts, _ := parseAppVersion(a)
	bParts, _ := parseAppVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

// parseAppVersion deliberately accepts only the SemVer core format. Prerelease
// and build metadata are not part of the mobile version-policy contract.
func parseAppVersion(version string) ([3]uint64, bool) {
	var parsed [3]uint64
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return parsed, false
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return parsed, false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return parsed, false
			}
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return parsed, false
		}
		parsed[i] = value
	}
	return parsed, true
}

func isValidSemver(version string) bool {
	_, valid := parseAppVersion(version)
	return valid
}

func validateMobileVersionRange(minimumVersion, latestVersion string) error {
	if minimumVersion != "" && latestVersion != "" && compareSemver(minimumVersion, latestVersion) > 0 {
		return errors.New("minimumVersion must not exceed latestVersion")
	}
	return nil
}

func evaluateMobileVersion(appVersion string, config MobileVersionConfig) CheckMobileVersionResponse {
	resp := CheckMobileVersionResponse{
		MinimumVersion: config.MinimumVersion,
		LatestVersion:  config.LatestVersion,
		UpdateUrl:      config.UpdateUrl,
		UpdateMessage:  config.UpdateMessage,
	}

	if config.Id == 0 {
		resp.Status = "ok"
	} else if config.MinimumVersion != "" && compareSemver(appVersion, config.MinimumVersion) < 0 {
		resp.Status = "update_required"
	} else if config.LatestVersion != "" && compareSemver(appVersion, config.LatestVersion) < 0 {
		resp.Status = "update_available"
	} else {
		resp.Status = "ok"
	}

	return resp
}

// mobileVersionPolicyHandler exposes version policy before authentication so a
// native client can decide whether it must update before presenting login. The
// response contains only operator-configured public store guidance.
func mobileVersionPolicyHandler(db *vbolt.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform := r.URL.Query().Get("platform")
		appVersion := r.URL.Query().Get("appVersion")
		if platform != "ios" && platform != "android" {
			http.Error(w, "platform must be 'ios' or 'android'", http.StatusBadRequest)
			return
		}
		if !isValidSemver(appVersion) {
			http.Error(w, "appVersion must be a valid major.minor.patch version", http.StatusBadRequest)
			return
		}

		var config MobileVersionConfig
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			vbolt.Read(tx, MobileVersionBkt, platformId(platform), &config)
		})

		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(evaluateMobileVersion(appVersion, config)); err != nil {
			http.Error(w, "failed to encode version policy", http.StatusInternalServerError)
		}
	}
}

// vbeam procedures

func CheckMobileVersion(ctx *vbeam.Context, req CheckMobileVersionRequest) (resp CheckMobileVersionResponse, err error) {
	_, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	if req.Platform != "ios" && req.Platform != "android" {
		err = errors.New("platform must be 'ios' or 'android'")
		return
	}
	if !isValidSemver(req.AppVersion) {
		err = errors.New("appVersion must be a valid semver string (e.g. 1.2.0)")
		return
	}

	id := platformId(req.Platform)
	var config MobileVersionConfig
	vbolt.Read(ctx.Tx, MobileVersionBkt, id, &config)
	resp = evaluateMobileVersion(req.AppVersion, config)

	return
}

func AdminSetMobileVersion(ctx *vbeam.Context, req AdminSetMobileVersionRequest) (resp AdminSetMobileVersionResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if user.Id != 1 {
		err = errors.New("admin access required")
		return
	}

	if req.Platform != "ios" && req.Platform != "android" {
		err = errors.New("platform must be 'ios' or 'android'")
		return
	}
	if req.MinimumVersion != "" && !isValidSemver(req.MinimumVersion) {
		err = errors.New("minimumVersion must be a valid semver string (e.g. 1.0.0)")
		return
	}
	if req.LatestVersion != "" && !isValidSemver(req.LatestVersion) {
		err = errors.New("latestVersion must be a valid semver string (e.g. 1.2.0)")
		return
	}
	if validationErr := validateMobileVersionRange(req.MinimumVersion, req.LatestVersion); validationErr != nil {
		err = validationErr
		return
	}

	id := platformId(req.Platform)
	config := MobileVersionConfig{
		Id:             id,
		Platform:       req.Platform,
		MinimumVersion: req.MinimumVersion,
		LatestVersion:  req.LatestVersion,
		UpdateUrl:      req.UpdateUrl,
		UpdateMessage:  req.UpdateMessage,
	}

	vbeam.UseWriteTx(ctx)
	vbolt.Write(ctx.Tx, MobileVersionBkt, id, &config)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}
