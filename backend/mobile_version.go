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

// AdminMobileVersionPlatform is one platform's stored policy as the admin page
// sees it, including what the server would refuse to serve.
type AdminMobileVersionPlatform struct {
	Platform       string `json:"platform"`
	Configured     bool   `json:"configured"`
	MinimumVersion string `json:"minimumVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateUrl      string `json:"updateUrl"`
	UpdateMessage  string `json:"updateMessage"`
	// AllowedHosts is the store-URL allowlist for this platform, so the page can
	// say what a valid URL looks like instead of making the operator guess.
	AllowedHosts []string `json:"allowedHosts"`
	// Warnings name stored values that would be rejected if saved today, and are
	// therefore withheld from clients. Rows predating these rules are still in
	// the bucket; this is how an operator finds out.
	Warnings []string `json:"warnings"`
}

type AdminGetMobileVersionsResponse struct {
	Platforms []AdminMobileVersionPlatform `json:"platforms"`
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

// The update URL and message are the only operator-authored strings this
// server hands to an unauthenticated caller, and a client that has been told it
// must update will follow the URL before anybody signs in. So neither is taken
// on trust: the URL has to point at a real store listing, and the message has
// to be a single short line of prose with no link of its own.

// storeHosts is the set of hosts an update URL may point at, per platform.
// TestFlight is included for iOS because a build can be forced forward during a
// beta, before there is an App Store listing to link to.
var storeHosts = map[string][]string{
	"ios":     {"apps.apple.com", "itunes.apple.com", "testflight.apple.com"},
	"android": {"play.google.com"},
}

const maxUpdateMessageLength = 200

// validateStoreUrl rejects anything that is not an https store listing for the
// platform being configured. A typo here does not fail loudly at the server —
// it sends every out-of-date install somewhere else.
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

// validateUpdateMessage keeps the forced-update text to one short line. Control
// characters would let it forge structure in whatever the client renders it
// into, and a URL in the text is a link the app cannot check against the store
// allowlist above.
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

// sanitizeUpdateGuidance drops guidance that would not pass validation today.
// Rows written before these rules existed are still in the bucket, and this is
// the last point before the values reach a client.
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

// AdminGetMobileVersions reports the stored policy for every platform. Setting
// a policy blind — with no way to read back what is live — is how a minimum
// version gets raised past a build nobody has yet.
func AdminGetMobileVersions(ctx *vbeam.Context, req Empty) (resp AdminGetMobileVersionsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if user.Id != 1 {
		err = errors.New("admin access required")
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
	// A prompt with nowhere to send the user is worse than no prompt: on a
	// mandatory update the app has already refused to continue.
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
