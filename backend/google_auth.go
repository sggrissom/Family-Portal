package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.hasen.dev/vbolt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Google iOS Client ID for mobile app token verification
var googleIOSClientID string

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

// GoogleTokenLoginRequest represents the request body for iOS Google Sign-In
type GoogleTokenLoginRequest struct {
	IDToken string `json:"idToken"`
}

// GoogleTokenInfo represents the response from Google's tokeninfo endpoint
type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Azp           string `json:"azp"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Iat           string `json:"iat"`
	Exp           string `json:"exp"`
	Alg           string `json:"alg"`
	Kid           string `json:"kid"`
	Typ           string `json:"typ"`
}

// SetupGoogleOAuth initializes the Google OAuth configuration
func SetupGoogleOAuth() error {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	siteRoot := os.Getenv("SITE_ROOT")

	// Optional iOS client ID for mobile app
	googleIOSClientID = os.Getenv("GOOGLE_IOS_CLIENT_ID")

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

// verifyGoogleIDToken verifies a Google ID token using Google's tokeninfo endpoint
func verifyGoogleIDToken(idToken string) (*GoogleTokenInfo, error) {
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token verification failed: %s", string(body))
	}

	var tokenInfo GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to decode token info: %v", err)
	}

	// Verify the token was issued by Google
	if tokenInfo.Iss != "accounts.google.com" && tokenInfo.Iss != "https://accounts.google.com" {
		return nil, errors.New("invalid token issuer")
	}

	// Verify the audience matches our client ID (web or iOS)
	webClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if tokenInfo.Aud != webClientID && tokenInfo.Aud != googleIOSClientID {
		return nil, errors.New("invalid token audience")
	}

	return &tokenInfo, nil
}

// googleTokenLoginHandler handles Google Sign-In from iOS using ID token
func googleTokenLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GoogleTokenLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid request body"})
		return
	}

	if req.IDToken == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "ID token is required"})
		return
	}

	// Verify the ID token with Google
	tokenInfo, err := verifyGoogleIDToken(req.IDToken)
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Google token verification failed", map[string]interface{}{
			"error": err.Error(),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid Google token"})
		return
	}

	// Check if user already exists
	var userId int
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		userId = GetUserId(tx, tokenInfo.Email)
	})

	var user User
	if userId > 0 {
		// User exists, get their info
		vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
			user = GetUser(tx, userId)
		})
	} else {
		// Create new user
		createAccountRequest := CreateAccountRequest{
			Name:            tokenInfo.Name,
			Email:           tokenInfo.Email,
			Password:        "",
			ConfirmPassword: "",
		}

		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			user = AddUserTx(tx, createAccountRequest, []byte{})
			vbolt.TxCommit(tx)
		})

		if user.Id == 0 {
			LogErrorWithRequest(r, LogCategoryAuth, "Failed to create user from Google sign-in", map[string]interface{}{
				"email": tokenInfo.Email,
			})
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to create account"})
			return
		}

		LogInfoWithRequest(r, LogCategoryAuth, "New user created via Google sign-in", map[string]interface{}{
			"userId": user.Id,
			"email":  user.Email,
		})
	}

	// Generate JWT and set cookies
	token, err := generateAuthJwt(user, w)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to generate JWT for Google sign-in", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to generate token"})
		return
	}

	LogInfoWithRequest(r, LogCategoryAuth, "Google sign-in successful", map[string]interface{}{
		"userId": user.Id,
		"email":  user.Email,
	})

	resp := GetAuthResponseFromUser(user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Token: token, Auth: resp})
}
