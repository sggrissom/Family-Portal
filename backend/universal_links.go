package backend

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"

	"go.hasen.dev/vbeam"
)

var iosAppIDPattern = regexp.MustCompile(`^[A-Z0-9]{10}\.[A-Za-z0-9][A-Za-z0-9.\-]*$`)

var universalLinkPaths = []string{
	"/chat",
	"/settings",
	"/photos",
	"/family-timeline",
	"/profile/*",
	"/person-activities/*",
}

var universalLinkExcludedPaths = []string{
	"/reset-password",
	"/api/*",
}

const AppSiteAssociationPath = "/.well-known/apple-app-site-association"

type appSiteAssociation struct {
	Applinks       appLinks           `json:"applinks"`
	WebCredentials *appWebCredentials `json:"webcredentials,omitempty"`
}

type appLinks struct {
	Apps    []string        `json:"apps"`
	Details []appLinkDetail `json:"details"`
}

type appLinkDetail struct {
	AppIDs     []string           `json:"appIDs"`
	Components []appLinkComponent `json:"components"`
}

type appLinkComponent struct {
	Path    string `json:"/"`
	Comment string `json:"comment,omitempty"`
	Exclude bool   `json:"exclude,omitempty"`
}

type appWebCredentials struct {
	Apps []string `json:"apps"`
}

func IOSAppID() string {
	return strings.TrimSpace(os.Getenv("IOS_APP_ID"))
}

func BuildAppSiteAssociation(appID string) appSiteAssociation {
	components := make([]appLinkComponent, 0, len(universalLinkExcludedPaths)+len(universalLinkPaths))

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

func RegisterUniversalLinkHandlers(app *vbeam.Application) {
	app.HandleFunc("GET "+AppSiteAssociationPath, appSiteAssociationHandler)
}

func appSiteAssociationHandler(w http.ResponseWriter, r *http.Request) {
	appID := IOSAppID()
	if !iosAppIDPattern.MatchString(appID) {
		http.NotFound(w, r)
		return
	}

	body, err := json.Marshal(BuildAppSiteAssociation(appID))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
