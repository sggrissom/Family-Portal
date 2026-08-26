package backend

import (
	"encoding/json"
	"errors"
	"family/cfg"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterMobileVersionMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CheckMobileVersion)
	vbeam.RegisterProc(app, AdminGetMobileVersions)
	vbeam.RegisterProc(app, AdminSetMobileVersion)
	app.HandleFunc("GET /api/mobile-version", mobileVersionPolicyHandler(app.DB))
}

type CheckMobileVersionRequest struct {
	Platform   string `json:"platform"`
	AppVersion string `json:"appVersion"`
}

type CheckMobileVersionResponse struct {
	Status         string `json:"status"`
	MinimumVersion string `json:"minimumVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateUrl      string `json:"updateUrl"`
	UpdateMessage  string `json:"updateMessage"`
}

type AdminSetMobileVersionRequest struct {
	Platform       string `json:"platform"`
	MinimumVersion string `json:"minimumVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateUrl      string `json:"updateUrl"`
	UpdateMessage  string `json:"updateMessage"`
}

type AdminSetMobileVersionResponse struct {
	Success bool `json:"success"`
}

type AdminMobileVersionPlatform struct {
	Platform       string   `json:"platform"`
	Configured     bool     `json:"configured"`
	MinimumVersion string   `json:"minimumVersion"`
	LatestVersion  string   `json:"latestVersion"`
	UpdateUrl      string   `json:"updateUrl"`
	UpdateMessage  string   `json:"updateMessage"`
	AllowedHosts   []string `json:"allowedHosts"`
	Warnings       []string `json:"warnings"`
}

type AdminGetMobileVersionsResponse struct {
	Platforms []AdminMobileVersionPlatform `json:"platforms"`
}

type MobileVersionConfig struct {
	Id             int
	Platform       string
	MinimumVersion string
	LatestVersion  string
	UpdateUrl      string
	UpdateMessage  string
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

var storeHosts = map[string][]string{
	"ios":     {"apps.apple.com", "itunes.apple.com", "testflight.apple.com"},
	"android": {"play.google.com"},
}

const maxUpdateMessageLength = 200

func validateStoreUrl(platform, rawUrl string) error {
	parsed, parseErr := url.Parse(rawUrl)
	if parseErr != nil {
		return errors.New("updateUrl must be a valid URL")
	}
	if parsed.Scheme != "https" {
		return errors.New("updateUrl must be an https URL")
	}
	if parsed.User != nil {
		return errors.New("updateUrl must not contain credentials")
	}
	hosts, known := storeHosts[platform]
	if !known {
		return errors.New("platform must be 'ios' or 'android'")
	}
	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range hosts {
		if host == allowed {
			return nil
		}
	}
	return errors.New("updateUrl must point at one of: " + strings.Join(hosts, ", "))
}

func validateUpdateMessage(message string) error {
	if len(message) > maxUpdateMessageLength {
		return errors.New("updateMessage must be at most " + strconv.Itoa(maxUpdateMessageLength) + " characters")
	}
	for _, char := range message {
		if char < 0x20 || char == 0x7f {
			return errors.New("updateMessage must be a single line of plain text")
		}
	}
	if strings.Contains(message, "://") {
		return errors.New("updateMessage must not contain a URL; use updateUrl for the store link")
	}
	return nil
}

func sanitizeUpdateGuidance(config MobileVersionConfig) (updateUrl, updateMessage string) {
	updateUrl = config.UpdateUrl
	updateMessage = config.UpdateMessage
	if updateUrl != "" && validateStoreUrl(config.Platform, updateUrl) != nil {
		updateUrl = ""
	}
	if validateUpdateMessage(updateMessage) != nil {
		updateMessage = ""
	}
	return
}

func validateMobileVersionRange(minimumVersion, latestVersion string) error {
	if minimumVersion != "" && latestVersion != "" && compareSemver(minimumVersion, latestVersion) > 0 {
		return errors.New("minimumVersion must not exceed latestVersion")
	}
	return nil
}

func evaluateMobileVersion(appVersion string, config MobileVersionConfig) CheckMobileVersionResponse {
	updateUrl, updateMessage := sanitizeUpdateGuidance(config)
	resp := CheckMobileVersionResponse{
		MinimumVersion: config.MinimumVersion,
		LatestVersion:  config.LatestVersion,
		UpdateUrl:      updateUrl,
		UpdateMessage:  updateMessage,
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

func AdminGetMobileVersions(ctx *vbeam.Context, req Empty) (resp AdminGetMobileVersionsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	resp.Platforms = []AdminMobileVersionPlatform{}
	for _, platform := range []string{"ios", "android"} {
		var config MobileVersionConfig
		configured := vbolt.Read(ctx.Tx, MobileVersionBkt, platformId(platform), &config)

		entry := AdminMobileVersionPlatform{
			Platform:       platform,
			Configured:     configured,
			MinimumVersion: config.MinimumVersion,
			LatestVersion:  config.LatestVersion,
			UpdateUrl:      config.UpdateUrl,
			UpdateMessage:  config.UpdateMessage,
			AllowedHosts:   storeHosts[platform],
			Warnings:       []string{},
		}
		if config.UpdateUrl != "" {
			if urlErr := validateStoreUrl(platform, config.UpdateUrl); urlErr != nil {
				entry.Warnings = append(entry.Warnings, "Stored update URL is withheld from clients: "+urlErr.Error()+".")
			}
		} else if config.MinimumVersion != "" || config.LatestVersion != "" {
			entry.Warnings = append(entry.Warnings, "A version policy is set with no update URL, so a prompted client has nowhere to go.")
		}
		if msgErr := validateUpdateMessage(config.UpdateMessage); msgErr != nil {
			entry.Warnings = append(entry.Warnings, "Stored update message is withheld from clients: "+msgErr.Error()+".")
		}

		resp.Platforms = append(resp.Platforms, entry)
	}

	return
}

func AdminSetMobileVersion(ctx *vbeam.Context, req AdminSetMobileVersionRequest) (resp AdminSetMobileVersionResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	if req.Platform != "ios" && req.Platform != "android" {
		err = errors.New("platform must be 'ios' or 'android'")
		return
	}

	minimumVersion := strings.TrimSpace(req.MinimumVersion)
	latestVersion := strings.TrimSpace(req.LatestVersion)
	updateUrl := strings.TrimSpace(req.UpdateUrl)
	updateMessage := strings.TrimSpace(req.UpdateMessage)

	if minimumVersion != "" && !isValidSemver(minimumVersion) {
		err = errors.New("minimumVersion must be a valid semver string (e.g. 1.0.0)")
		return
	}
	if latestVersion != "" && !isValidSemver(latestVersion) {
		err = errors.New("latestVersion must be a valid semver string (e.g. 1.2.0)")
		return
	}
	if validationErr := validateMobileVersionRange(minimumVersion, latestVersion); validationErr != nil {
		err = validationErr
		return
	}
	if updateUrl != "" {
		if validationErr := validateStoreUrl(req.Platform, updateUrl); validationErr != nil {
			err = validationErr
			return
		}
	}
	if updateUrl == "" && (minimumVersion != "" || latestVersion != "") {
		err = errors.New("updateUrl is required once a minimum or latest version is set")
		return
	}
	if validationErr := validateUpdateMessage(updateMessage); validationErr != nil {
		err = validationErr
		return
	}

	id := platformId(req.Platform)
	config := MobileVersionConfig{
		Id:             id,
		Platform:       req.Platform,
		MinimumVersion: minimumVersion,
		LatestVersion:  latestVersion,
		UpdateUrl:      updateUrl,
		UpdateMessage:  updateMessage,
	}

	vbeam.UseWriteTx(ctx)
	vbolt.Write(ctx.Tx, MobileVersionBkt, id, &config)
	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}
