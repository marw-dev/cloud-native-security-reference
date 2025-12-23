package auth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID	string 		`json:"user_id"`
	Roles 	[]string 	`json:"roles"`
	jwt.RegisteredClaims
}

type UserDataKey string

const ContextKey UserDataKey = "UserData"


func ValidateToken(tokenString string, publicKey *rsa.PublicKey) (*CustomClaims, error) {

	if publicKey == nil {
		return nil, fmt.Errorf("interner fehler: öffentlicher schlüssel nicht geladen")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unerwarteter Signaturalgorithmus: %v, erwartet RS256", token.Header["alg"])
		}
		return publicKey, nil
	}, jwt.WithLeeway(5*time.Second))

	if err != nil {
		return nil, fmt.Errorf("token Validierungsfehler: %w", err)
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("ungültiges oder abgelaufenes Token")
}

func AuthMiddleware(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Fehlendes oder ungültiges 'Authorization: Bearer' Header", http.StatusUnauthorized)
				return
			}
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			claims, err := ValidateToken(tokenString, publicKey)
			if err != nil {
				http.Error(w, fmt.Sprintf("Authentifizierung fehlgeschlagen: %v", err), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKey, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ACLMiddleware(requiredRoles []string) func(http.Handler) http.Handler {
	if len(requiredRoles) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(ContextKey).(*CustomClaims)

			if !ok {
				http.Error(w, "Autorisierung fehlgeschlagen: Kontextdaten fehlen.", http.StatusForbidden)
				return
			}

			hasRequiredRole := false
			userRolesMap := make(map[string]bool)

			for _, role := range claims.Roles {
				userRolesMap[role] = true
			}

			for _, requiredRole := range requiredRoles {
				if userRolesMap[requiredRole] {
					hasRequiredRole = true
					break
				}
			}

			if !hasRequiredRole {
				http.Error(w, "Zugriff verweigert. Fehlende Rollenberechtigung.", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
