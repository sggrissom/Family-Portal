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
		token, err := generateToken(32)
		if err != nil {
			log.Fatal("error generating JWT secret")
		}
		jwtSecret = token
		log.Println("Generated JWT secret. Set JWT_SECRET_KEY environment variable for production.")
	}
	jwtKey = []byte(jwtSecret)

	// Register essential auth API endpoints
	app.HandleFunc("/api/login", loginHandler)
	app.HandleFunc("/api/logout", logoutHandler)

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
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid credentials"})
		return
	}

	err := bcrypt.CompareHashAndPassword(passHash, []byte(credentials.Password))
	if err != nil {
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Invalid credentials"})
		return
	}

	token, err := generateAuthJwt(user, w)
	if err != nil {
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "Failed to generate token"})
		return
	}

	resp := GetAuthResponseFromUser(user)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Token: token, Auth: resp})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "authToken",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})
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

	// Update last login
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		user.LastLogin = time.Now()
		vbolt.Write(tx, UsersBkt, user.Id, &user)
		vbolt.TxCommit(tx)
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
