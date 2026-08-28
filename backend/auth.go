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
	"strings"
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
	DeviceToken  string `json:"deviceToken"`
	RefreshToken string `json:"refreshToken"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

var appDb *vbolt.DB

const minimumJWTSecretLength = 32

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

	app.HandleFunc("/api/login", loginHandler)
	app.HandleFunc("/api/logout", logoutHandler)
	app.HandleFunc("/api/refresh", refreshTokenHandler)

	app.HandleFunc("/api/login/google", googleLoginHandler)
	app.HandleFunc("/api/google/callback", googleCallbackHandler)
	app.HandleFunc("/api/login/google/token", googleTokenLoginHandler)

	app.HandleFunc("/api/login/apple", appleLoginHandler)
	app.HandleFunc("/api/apple/callback", appleCallbackHandler)
	app.HandleFunc("/api/login/apple/token", appleTokenLoginHandler)

	app.HandleFunc("/api/auth/providers", authProvidersHandler)

	err = SetupGoogleOAuth()
	if err != nil {
		log.Printf("Google OAuth setup failed: %v", err)
		log.Println("Google login will not be available. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to enable.")
	}

	err = SetupAppleOAuth()
	if err != nil {
		log.Printf("Apple Sign In setup failed: %v", err)
		log.Println("Apple login will not be available. Set APPLE_CLIENT_ID, APPLE_TEAM_ID, APPLE_KEY_ID, and APPLE_KEY_PATH to enable.")
	}

	appDb = app.DB
}

const invalidCredentialsMessage = "Invalid credentials"

var decoyPasswordHash = sync.OnceValue(func() []byte {
	secret, err := generateToken(32)
	if err != nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}
	return hash
})

// Burns the same bcrypt work a real check would, so a missing account and a
// wrong password take the same time. The result can never match, hence discarded.
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
		compareAgainstDecoyPassword(credentials.Password)
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

	user, _ := AuthenticateRequest(r)

	var logoutRequest LogoutRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&logoutRequest); err != nil && !errors.Is(err, io.EOF) {
			vbeam.RespondError(w, errors.New("invalid logout request"))
			return
		}
	}

	if presented, _ := presentedRefreshToken(r, logoutRequest.RefreshToken); presented != "" {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			DeleteRefreshToken(tx, presented)
			vbolt.TxCommit(tx)
		})
	}

	if user.Id != 0 && logoutRequest.DeviceToken != "" {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			if err := deactivatePushDeviceToken(tx, user.Id, logoutRequest.DeviceToken); err == nil {
				vbolt.TxCommit(tx)
			}
		})
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})

	clearRefreshTokenCookie(w)

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

func setAuthJwtCookie(user User, w http.ResponseWriter) (tokenString string, err error) {
	expirationTime := time.Now().Add(24 * time.Hour)
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
		MaxAge:   60 * 60 * 24,
	})
	return
}

func generateAuthJwt(user User, w http.ResponseWriter) (tokenString string, err error) {
	tokenString, err = setAuthJwtCookie(user, w)
	if err != nil {
		return
	}

	var refreshTokenString string
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		_, refreshTokenString, err = CreateRefreshToken(tx, user.Id, refreshTokenLifetime)
		if err != nil {
			return
		}

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

	var refreshRequest RefreshRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&refreshRequest); err != nil && !errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid refresh request",
			})
			return
		}
	}

	presented, viaCookie := presentedRefreshToken(r, refreshRequest.RefreshToken)

	if presented == "" {
		LogWarnWithRequest(r, LogCategoryAuth, "Refresh attempt without token", nil)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No refresh token provided",
		})
		return
	}

	var user User
	var rotated RefreshToken
	var newTokenString string
	var rotateErr error
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		rotated, newTokenString, rotateErr = RotateRefreshToken(tx, presented, time.Now())
		if rotateErr != nil {
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

	LogInfoWithRequest(r, LogCategoryAuth, "Token refresh successful", map[string]interface{}{
		"userId": user.Id,
		"email":  redactEmail(user.Email),
	})

	w.Header().Set("Content-Type", "application/json")
	resp := GetAuthResponseForUser(user)
	body := map[string]interface{}{
		"success": true,
		"token":   token,
		"auth":    resp,
	}
	if !viaCookie {
		body["refreshToken"] = newTokenString
	}
	json.NewEncoder(w).Encode(body)
}

func presentedRefreshToken(r *http.Request, fromBody string) (token string, viaCookie bool) {
	if cookie, err := r.Cookie("refreshToken"); err == nil && cookie.Value != "" {
		return cookie.Value, true
	}
	return strings.TrimSpace(fromBody), false
}
