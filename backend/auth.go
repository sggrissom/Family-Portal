package backend

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
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

var appDb *vbolt.DB

func SetupAuth(app *vbeam.Application) {
	// Get JWT secret from environment, generate one if not set
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		// Check if this is production (could be detected by other env vars)
		if os.Getenv("ENVIRONMENT") == "production" || os.Getenv("PROD") == "true" {
			log.Fatal("JWT_SECRET_KEY must be set in production environment")
		}

		token, err := generateToken(32)
		if err != nil {
			log.Fatal("error generating JWT secret")
		}
		jwtSecret = token
		log.Println("Generated JWT secret. Set JWT_SECRET_KEY environment variable for production.")
	}

	// Validate JWT secret strength
	if len(jwtSecret) < 16 {
		log.Fatal("JWT secret must be at least 16 characters long")
	}

	jwtKey = []byte(jwtSecret)

	// Register essential auth API endpoints
	app.HandleFunc("/api/login", loginHandler)
	app.HandleFunc("/api/logout", logoutHandler)
	app.HandleFunc("/api/refresh", refreshTokenHandler)

	// Register Google OAuth endpoints
	app.HandleFunc("/api/login/google", googleLoginHandler)
	app.HandleFunc("/api/google/callback", googleCallbackHandler)

	// Setup Google OAuth configuration
	err := SetupGoogleOAuth()
	if err != nil {
		log.Printf("Google OAuth setup failed: %v", err)
		log.Println("Google login will not be available. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to enable.")
	}

	appDb = app.DB
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

	if user.Id == 0 {
		LogWarnWithRequest(r, LogCategoryAuth, "Login attempt with unknown email", map[string]interface{}{
			"email": credentials.Email,
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid credentials"})
		return
	}

	err := bcrypt.CompareHashAndPassword(passHash, []byte(credentials.Password))
	if err != nil {
		LogWarnWithRequest(r, LogCategoryAuth, "Login attempt with invalid password", map[string]interface{}{
			"userId": user.Id,
			"email":  user.Email,
		})
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid credentials"})
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
		"email":  user.Email,
	})

	resp := GetAuthResponseFromUser(user)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Token: token, Auth: resp})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Try to get user info before clearing the cookie
	user, _ := AuthenticateRequest(r)

	// Delete refresh token from database if present
	if cookie, err := r.Cookie("refreshToken"); err == nil && cookie.Value != "" {
		vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
			DeleteRefreshToken(tx, cookie.Value)
			vbolt.TxCommit(tx)
		})
	}

	// Clear auth token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})

	// Clear refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})

	// Log logout event
	if user.Id != 0 {
		LogInfoWithRequest(r, LogCategoryAuth, "User logout", map[string]interface{}{
			"userId": user.Id,
			"email":  user.Email,
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

func generateAuthJwt(user User, w http.ResponseWriter) (tokenString string, err error) {
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
		MaxAge:   60 * 60 * 24, // 24 hours
	})

	// Create and set refresh token (30 days)
	var refreshToken RefreshToken
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		refreshToken, err = CreateRefreshToken(tx, user.Id, 30*24*time.Hour)
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

	// Set refresh token cookie (30 days)
	http.SetCookie(w, &http.Cookie{
		Name:     "refreshToken",
		Value:    refreshToken.Token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   60 * 60 * 24 * 30, // 30 days
	})

	return
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

	var user User
	var validToken RefreshToken

	// Validate refresh token and get user
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		var valid bool
		validToken, valid = ValidateRefreshToken(tx, cookie.Value)
		if !valid {
			return
		}

		user = GetUser(tx, validToken.UserId)
	})

	if user.Id == 0 {
		LogWarnWithRequest(r, LogCategoryAuth, "Refresh attempt with invalid token", nil)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid or expired refresh token",
		})
		return
	}

	// Update last used timestamp
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		UpdateRefreshTokenLastUsed(tx, validToken.Id)
		vbolt.TxCommit(tx)
	})

	// Generate new JWT
	token, err := generateAuthJwt(user, w)
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
		"email":  user.Email,
	})

	w.Header().Set("Content-Type", "application/json")
	resp := GetAuthResponseFromUser(user)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"auth":    resp,
	})
}
