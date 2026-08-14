package backend

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey []byte
var ErrLoginFailure = errors.New("LoginFailure")
var ErrAuthFailure = errors.New("AuthFailure")

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type LogoutRequest struct {
	DeviceToken string `json:"deviceToken"`
}

var appDb *vbolt.DB

const minimumJWTSecretLength = 32

// refreshTokenLifetime is how long a login lasts. Rotation issues successors
// that inherit this deadline rather than extending it, so a session ends a month
// after the login that started it however often it is refreshed.
const refreshTokenLifetime = 30 * 24 * time.Hour

func resolveJWTSecret() (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		if cfg.IsRelease {
			return "", errors.New("JWT_SECRET_KEY must be set in release builds")
		}

		token, err := generateToken(32)
		if err != nil {
			return "", errors.New("generate JWT secret")
		}
		log.Println("Generated JWT secret. Set JWT_SECRET_KEY environment variable for production.")
		return token, nil
	}

	if len(jwtSecret) < minimumJWTSecretLength {
		return "", fmt.Errorf("JWT_SECRET_KEY must be at least %d characters long", minimumJWTSecretLength)
	}

	return jwtSecret, nil
}

func SetupAuth(app *vbeam.Application) {
	jwtSecret, err := resolveJWTSecret()
	if err != nil {
		log.Fatal(err)
	}

	jwtKey = []byte(jwtSecret)

	// Register essential auth API endpoints
	app.HandleFunc("/api/login", loginHandler)
	app.HandleFunc("/api/logout", logoutHandler)
	app.HandleFunc("/api/refresh", refreshTokenHandler)

	// Register Google OAuth endpoints
	app.HandleFunc("/api/login/google", googleLoginHandler)
	app.HandleFunc("/api/google/callback", googleCallbackHandler)
	app.HandleFunc("/api/login/google/token", googleTokenLoginHandler)

	// Setup Google OAuth configuration
	err = SetupGoogleOAuth()
	if err != nil {
		log.Printf("Google OAuth setup failed: %v", err)
		log.Println("Google login will not be available. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to enable.")
	}

	appDb = app.DB
}

// invalidCredentialsMessage is the only answer a failed login gets. Wrong
// password, unknown address, and an account that only signs in with Google all
// produce this exact string, because any difference between them tells an
// anonymous caller which addresses have accounts here.
const invalidCredentialsMessage = "Invalid credentials"

// decoyPasswordHash is a bcrypt hash of a random value nobody can supply. It
// exists to be compared against, so that a login for an address with no account
// costs the same time as one for an address with an account.
var decoyPasswordHash = sync.OnceValue(func() []byte {
	secret, err := generateToken(32)
	if err != nil {
		// A decoy that cannot be built must not take logins down with it; the
		// worst case is that timing stops being equalized.
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}
	return hash
})

// compareAgainstDecoyPassword spends the same work a real password check would.
// The result is deliberately discarded: it can never match.
func compareAgainstDecoyPassword(password string) {
	hash := decoyPasswordHash()
	if hash == nil {
		return
	}
	_ = bcrypt.CompareHashAndPassword(hash, []byte(password))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		vbeam.RespondError(w, errors.New("login call must be POST"))
		return
	}

	var credentials LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		vbeam.RespondError(w, ErrLoginFailure)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var user User
	var passHash []byte

	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		userId := GetUserId(tx, credentials.Email)
		if userId == 0 {
			return
		}
		user = GetUser(tx, userId)
		passHash = GetPassHash(tx, userId)
	})

	if user.Id == 0 || len(passHash) == 0 {
		// Hash the submitted password against a decoy anyway. Without this, a
		// missing account answers in microseconds and a real one takes as long
		// as bcrypt does, which turns the login form into a directory of who
		// has an account here. The empty-hash case is a Google-only account,
		// which must be indistinguishable for the same reason.
		compareAgainstDecoyPassword(credentials.Password)
		// The response cannot say which of the two happened, but the log can.
		LogWarnWithRequest(r, LogCategoryAuth, "Login attempt with no usable password on file", map[string]interface{}{
			"email":        redactEmail(credentials.Email),
			"accountFound": user.Id != 0,
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: invalidCredentialsMessage})
		return
	}

	err := bcrypt.CompareHashAndPassword(passHash, []byte(credentials.Password))
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Login attempt with invalid password", map[string]interface{}{
			"userId": user.Id,
			"email":  redactEmail(user.Email),
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: invalidCredentialsMessage})
		return
	}

	token, err := generateAuthJwt(user, w)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to generate JWT token", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to generate token"})
		return
	}

	// Log successful login
	LogInfoWithRequest(r, LogCategoryAuth, "User login successful", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	resp := GetAuthResponseForUser(user)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Token: token, Auth: resp})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		vbeam.RespondError(w, errors.New("logout call must be POST"))
		return
	}

	// Try to get user info before clearing the cookie
	user, _ := AuthenticateRequest(r)

	// Native clients can identify the device that is signing out so it no
	// longer receives notifications for this account. An empty body remains
	// valid for browser clients, which do not register a push device token.
	var logoutRequest LogoutRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&logoutRequest); err != nil && !errors.Is(err, io.EOF) {
			vbeam.RespondError(w, errors.New("invalid logout request"))
			return
		}
	}

	// Delete refresh token from database if present
	if cookie, err := r.Cookie("refreshToken"); err == nil && cookie.Value != "" {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			DeleteRefreshToken(tx, cookie.Value)
			vbolt.TxCommit(tx)
		})
	}

	if user.Id != 0 && logoutRequest.DeviceToken != "" {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			// Do not disclose whether an arbitrary token exists or belongs to a
			// different user. Logout should still succeed and clear the session.
			if err := deactivatePushDeviceToken(tx, user.Id, logoutRequest.DeviceToken); err == nil {
				vbolt.TxCommit(tx)
			}
		})
	}

	// Clear auth token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})

	// Clear refresh token cookie
	clearRefreshTokenCookie(w)

	// Log logout event
	if user.Id != 0 {
		LogInfoWithRequest(r, LogCategoryAuth, "User logout", map[string]interface{}{
			"userId": user.Id,
			"email":  redactEmail(user.Email),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// setAuthJwtCookie generates a JWT token and sets it as a cookie
func setAuthJwtCookie(user User, w http.ResponseWriter) (tokenString string, err error) {
	expirationTime := time.Now().Add(24 * time.Hour) // 24 hour expiry
	claims := &Claims{
		Username: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err = token.SignedString(jwtKey)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   60 * 60 * 24, // 24 hours
	})
	return
}

func generateAuthJwt(user User, w http.ResponseWriter) (tokenString string, err error) {
	tokenString, err = setAuthJwtCookie(user, w)
	if err != nil {
		return
	}

	// Create and set refresh token (30 days)
	var refreshTokenString string
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		_, refreshTokenString, err = CreateRefreshToken(tx, user.Id, refreshTokenLifetime)
		if err != nil {
			return
		}

		// Update last login
		user.LastLogin = time.Now()
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.TxCommit(tx)
	})

	if err != nil {
		return
	}

	setRefreshTokenCookie(w, refreshTokenString)

	return
}

// setRefreshTokenCookie writes the refresh cookie. Rotation means this happens
// on every refresh, not only at login, so both paths go through here and cannot
// drift apart in attributes.
func setRefreshTokenCookie(w http.ResponseWriter, tokenString string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(refreshTokenLifetime.Seconds()),
	})
}

// clearRefreshTokenCookie removes the refresh cookie from the browser.
func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

func generateJwtTokenString(user User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func GetAuthUser(ctx *vbeam.Context) (user User, err error) {
	if len(ctx.Token) == 0 {
		return user, ErrAuthFailure
	}
	token, err := jwt.ParseWithClaims(ctx.Token, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return
	}

	if claims, ok := token.Claims.(*Claims); ok {
		user = GetUser(ctx.Tx, GetUserId(ctx.Tx, claims.Username))
	}
	return
}

func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		vbeam.RespondError(w, errors.New("refresh call must be POST"))
		return
	}

	// Get refresh token from cookie
	cookie, err := r.Cookie("refreshToken")
	if err != nil || cookie.Value == "" {
		LogWarnWithRequest(r, LogCategoryAuth, "Refresh attempt without token", nil)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No refresh token provided",
		})
		return
	}

	// Exchange the presented token for a fresh one. The rotation and the user
	// lookup share a write transaction so a token cannot be spent twice by two
	// requests arriving together.
	var user User
	var rotated RefreshToken
	var newTokenString string
	var rotateErr error
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		rotated, newTokenString, rotateErr = RotateRefreshToken(tx, cookie.Value, time.Now())
		if rotateErr != nil {
			// The reuse case revokes the session, which is a write worth
			// keeping even though the request fails.
			vbolt.TxCommit(tx)
			return
		}

		user = GetUser(tx, rotated.UserId)
		if user.Id == 0 {
			return
		}
		vbolt.TxCommit(tx)
	})

	if errors.Is(rotateErr, ErrRefreshTokenReused) {
		// Either the token was stolen and replayed, or the legitimate holder
		// kept using a token after its successor was issued. Both end the
		// session; the log line is what makes the first case investigable.
		LogWarnWithRequest(r, LogCategoryAuth, "Refresh token reuse detected; session revoked", nil)
		clearRefreshTokenCookie(w)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid or expired refresh token",
		})
		return
	}

	if rotateErr != nil || user.Id == 0 {
		LogWarnWithRequest(r, LogCategoryAuth, "Refresh attempt with invalid token", nil)
		clearRefreshTokenCookie(w)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid or expired refresh token",
		})
		return
	}

	setRefreshTokenCookie(w, newTokenString)

	// Generate new JWT (uses helper function that doesn't create new refresh token)
	token, err := setAuthJwtCookie(user, w)
	if err != nil {
		LogErrorWithRequest(r, LogCategoryAuth, "Failed to generate JWT during refresh", map[string]interface{}{
			"userId": user.Id,
			"error":  err.Error(),
		})
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to generate token",
		})
		return
	}

	// Log successful refresh
	LogInfoWithRequest(r, LogCategoryAuth, "Token refresh successful", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	w.Header().Set("Content-Type", "application/json")
	resp := GetAuthResponseForUser(user)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"auth":    resp,
	})
}
