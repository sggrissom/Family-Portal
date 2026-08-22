package backend

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"

	"go.hasen.dev/vbeam"
)

// A universal link is the same URL twice: a web page for anyone who follows it
// in a browser, and a screen in the companion app for anyone who follows it on a
// device with the app installed. The push payload already speaks in these terms
// — every event spec in push_worker.go carries a site-relative Destination that
// matches the web route for the same content — but nothing served the file iOS
// needs in order to honor them, so tapping a notification could only ever open
// the app's last screen.
//
// The file is /.well-known/apple-app-site-association. Apple's CDN fetches it
// over https, without redirects, and hands the result to the device at install
// time; the device then intercepts exactly the paths it names. Three properties
// of that arrangement drive the decisions below:
//
//  1. It is cached hard, by Apple and by the device. A wrong appID or an
//     over-broad path list is not a bug you fix with a deploy — it is one you
//     wait out. So an unconfigured server serves nothing at all rather than
//     something plausible, and the path list is an allowlist of routes the app
//     genuinely has a screen for.
//
//  2. Anything it claims stops reaching Safari. A path listed here that the app
//     cannot open is a link that goes nowhere. The password-reset and OAuth
//     callback routes are the sharp end of this: both arrive from outside the
//     app, both must land in a browser, and both are excluded below.
//
//  3. It is public and unauthenticated by construction. It names an app and a
//     set of paths and nothing else — no user data reaches it, which is why it
//     can sit outside every auth check in the application.

// iosAppIDPattern matches Apple's `<TeamID>.<BundleID>` app identifier. The team
// id is a fixed ten-character alphanumeric prefix; the bundle id is reverse-DNS.
var iosAppIDPattern = regexp.MustCompile(`^[A-Z0-9]{10}\.[A-Za-z0-9][A-Za-z0-9.\-]*$`)

// universalLinkPaths are the routes the iOS app can open, in Apple's matching
// syntax. Anything absent from this list keeps working as an ordinary web link,
// which is the right outcome for every route the app has no screen for.
//
// Each entry is paired with the screen that honors it. Adding a path here
// without adding that screen produces a link that opens the app and then does
// nothing, which is worse than the browser.
var universalLinkPaths = []string{
	"/chat",                // ChatView — the destination of every chat push
	"/settings",            // SettingsView — the destination of a test push
	"/photos",              // PhotoGalleryView
	"/family-timeline",     // TimelineView
	"/profile/*",           // PersonDetailView, by remote person id
	"/person-activities/*", // PersonSeasonView, by remote person id
}

// universalLinkExcludedPaths never open the app, whatever else matches. They are
// listed explicitly, ahead of the allowed paths, because the cost of getting one
// of them wrong is a user who cannot finish the flow at all:
//
//   - /reset-password carries a single-use token from an email and is answered
//     by a page the app does not have.
//   - /api/* includes the Google OAuth callback, which must complete in the
//     browser session that started it.
//
// Neither is matched by the allowlist above today. They are here so that a path
// added to that list later — "/api/*" is exactly the kind of shortcut someone
// reaches for — cannot silently swallow them.
var universalLinkExcludedPaths = []string{
	"/reset-password",
	"/api/*",
}

// AppSiteAssociationPath is where Apple looks. It is not configurable.
const AppSiteAssociationPath = "/.well-known/apple-app-site-association"

// appSiteAssociation is the document's shape. The field names are Apple's.
type appSiteAssociation struct {
	Applinks       appLinks           `json:"applinks"`
	WebCredentials *appWebCredentials `json:"webcredentials,omitempty"`
}

type appLinks struct {
	// Details is the modern form: one entry per app, each with its own
	// components list. The older `apps` key must still be present and empty.
	Apps    []string        `json:"apps"`
	Details []appLinkDetail `json:"details"`
}

type appLinkDetail struct {
	AppIDs     []string           `json:"appIDs"`
	Components []appLinkComponent `json:"components"`
}

// appLinkComponent is one matching rule. Exclude is emitted only when set, so an
// ordinary allow rule stays a two-key object.
type appLinkComponent struct {
	Path    string `json:"/"`
	Comment string `json:"comment,omitempty"`
	Exclude bool   `json:"exclude,omitempty"`
}

// appWebCredentials lets the app's sign-in screen offer the password already
// saved for the website. It costs one key and removes the most common reason a
// beta tester gives up at the login form.
type appWebCredentials struct {
	Apps []string `json:"apps"`
}

// IOSAppID returns the configured `<TeamID>.<BundleID>`, or an empty string when
// universal links are not configured. Whitespace is trimmed because this value
// is pasted out of App Store Connect more often than it is typed.
func IOSAppID() string {
	return strings.TrimSpace(os.Getenv("IOS_APP_ID"))
}

// BuildAppSiteAssociation renders the document for one app identifier.
func BuildAppSiteAssociation(appID string) appSiteAssociation {
	components := make([]appLinkComponent, 0, len(universalLinkExcludedPaths)+len(universalLinkPaths))

	// Order is significant: iOS takes the first component that matches, so the
	// exclusions have to precede everything.
	for _, path := range universalLinkExcludedPaths {
		components = append(components, appLinkComponent{
			Path:    path,
			Exclude: true,
			Comment: "handled by the website only",
		})
	}
	for _, path := range universalLinkPaths {
		components = append(components, appLinkComponent{Path: path})
	}

	return appSiteAssociation{
		Applinks: appLinks{
			Apps: []string{},
			Details: []appLinkDetail{{
				AppIDs:     []string{appID},
				Components: components,
			}},
		},
		WebCredentials: &appWebCredentials{Apps: []string{appID}},
	}
}

// RegisterUniversalLinkHandlers wires the association file. It is registered
// unconditionally so that the handler, not the routing table, decides what an
// unconfigured server says — a 404 either way, but one that a test can reach.
func RegisterUniversalLinkHandlers(app *vbeam.Application) {
	app.HandleFunc("GET "+AppSiteAssociationPath, appSiteAssociationHandler)
}

func appSiteAssociationHandler(w http.ResponseWriter, r *http.Request) {
	appID := IOSAppID()
	if !iosAppIDPattern.MatchString(appID) {
		// Including the unset case. A malformed id is refused rather than
		// served, because an association naming the wrong app is cached by
		// Apple and by every device that fetched it.
		http.NotFound(w, r)
		return
	}

	body, err := json.Marshal(BuildAppSiteAssociation(appID))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Apple requires application/json and no redirect. The cache window is short
	// on purpose: Apple's CDN holds its own copy for far longer, and this is the
	// only lever available when the path list turns out to be wrong.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
