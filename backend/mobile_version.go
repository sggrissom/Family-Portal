package backend

import (
	"errors"
	"family/cfg"
	"strconv"
	"strings"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterMobileVersionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CheckMobileVersion)
	vbeam.RegisterProc(app, AdminSetMobileVersion)
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

// compareSemver compares two semver strings (major.minor.patch).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareSemver(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)

	for i := 0; i < 3; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			aNum, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bNum, _ = strconv.Atoi(bParts[i])
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}

func isValidSemver(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
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

	if config.Id == 0 {
		resp.Status = "ok"
		return
	}

	resp.MinimumVersion = config.MinimumVersion
	resp.LatestVersion = config.LatestVersion
	resp.UpdateUrl = config.UpdateUrl
	resp.UpdateMessage = config.UpdateMessage

	if config.MinimumVersion != "" && compareSemver(req.AppVersion, config.MinimumVersion) < 0 {
		resp.Status = "update_required"
	} else if config.LatestVersion != "" && compareSemver(req.AppVersion, config.LatestVersion) < 0 {
		resp.Status = "update_available"
	} else {
		resp.Status = "ok"
	}

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
