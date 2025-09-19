package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"go.hasen.dev/vbolt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var oauthConf *oauth2.Config
var oauthStateString string

// UserInfo represents the user information returned by Google OAuth
type UserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// SetupGoogleOAuth initializes the Google OAuth configuration
func SetupGoogleOAuth() error {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	siteRoot := os.Getenv("SITE_ROOT")

	if clientID == "" || clientSecret == "" {
		return errors.New("Google OAuth credentials not configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables")
	}

	if siteRoot == "" {
		siteRoot = "http://localhost:8666" // Default for local development
	}

	oauthConf = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  siteRoot + "/api/google/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// Generate a random state string for OAuth security
	token, err := generateToken(20)
	if err != nil {
		return fmt.Errorf("error generating OAuth state token: %v", err)
	}
	oauthStateString = token

	return nil
}

// googleLoginHandler redirects the user to Google's OAuth page
func googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	if oauthConf == nil {
		http.Error(w, "Google OAuth not configured", http.StatusInternalServerError)
		return
	}

	url := oauthConf.AuthCodeURL(oauthStateString, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// googleCallbackHandler processes the OAuth callback from Google
func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if oauthConf == nil {
		http.Error(w, "Google OAuth not configured", http.StatusInternalServerError)
		return
	}

	// Verify the state parameter to prevent CSRF attacks
	if r.FormValue("state") != oauthStateString {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for a token
	code := r.FormValue("code")
	token, err := oauthConf.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Code exchange failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Use the token to get user information from Google
	client := oauthConf.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed getting user info: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Parse the user information
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, fmt.Sprintf("Failed decoding user info: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Check if the user already exists in our database
	var userId int
	vbolt.WithReadTx(appDb, func(readTx *vbolt.Tx) {
		userId = GetUserId(readTx, userInfo.Email)
	})

	if userId > 0 {
		// User exists, authenticate them
		err = authenticateForUser(userId, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authentication failed: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	} else {
		// User doesn't exist, create a new account
		createAccountRequest := CreateAccountRequest{
			Name:            userInfo.Name,
			Email:           userInfo.Email,
			Password:        "",
			ConfirmPassword: "",
		}

		var user User
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			user = AddUserTx(tx, createAccountRequest, []byte{})
			vbolt.TxCommit(tx)
		})

		if user.Id > 0 {
			err = authenticateForUser(user.Id, w)
			if err != nil {
				http.Error(w, fmt.Sprintf("Authentication failed: %s", err.Error()), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Failed to create user account", http.StatusInternalServerError)
			return
		}
	}

	// Redirect to the dashboard after successful authentication
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// authenticateForUser generates JWT tokens and sets cookies for the given user ID
func authenticateForUser(userId int, w http.ResponseWriter) error {
	var user User
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		user = GetUser(tx, userId)
	})

	if user.Id == 0 {
		return errors.New("user not found")
	}

	// Generate and set JWT token
	_, err := generateAuthJwt(user, w)
	if err != nil {
		return fmt.Errorf("failed to generate auth token: %v", err)
	}

	return nil
}
