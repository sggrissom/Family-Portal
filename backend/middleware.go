package backend

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.hasen.dev/vbolt"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := AuthenticateRequest(r)
		if err != nil {
			RespondAuthError(w, r, "Authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func AuthenticateRequest(r *http.Request) (User, error) {
	var user User

	token := extractToken(r)
	if token == "" {
		return user, errors.New("no auth token found")
	}

	jwtToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtKey, nil
	})

	if err != nil || !jwtToken.Valid {
		return user, errors.New("invalid token")
	}

	claims, ok := jwtToken.Claims.(*Claims)
	if !ok {
		return user, errors.New("invalid claims")
	}

	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		userId := GetUserId(tx, claims.Username)
		if userId != 0 {
			user = GetUser(tx, userId)
		}
	})

	if user.Id == 0 {
		return user, errors.New("user not found")
	}

	return user, nil
}

func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie("authToken"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	bearerToken := r.Header.Get("Authorization")
	if len(bearerToken) > 7 && strings.ToUpper(bearerToken[0:7]) == "BEARER " {
		return bearerToken[7:]
	}

	return ""
}

func GetUserFromContext(r *http.Request) (User, bool) {
	user, ok := r.Context().Value(UserContextKey).(User)
	return user, ok
}

func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok {
			RespondAuthError(w, r, "Authentication required")
			return
		}

		if user.Id != AdminUserId {
			RespondForbiddenError(w, r, "Admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}
