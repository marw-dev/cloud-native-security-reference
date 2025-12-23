package middleware

import (
	"athena/internal/auth"
	"context"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDContextKey contextKey = "userID"
const ProjectIDContextKey contextKey = "projectID"

func Authenticator(publicKey *rsa.PublicKey) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tokenString, err := extractBearerToken(r)
			if err != nil {
				slog.WarnContext(ctx, "Authentifizierung fehlgeschlagen: Kein oder ungültiger Token", slog.Any("error", err))
				http.Error(w, `{"error": "unauthorized", "message": "Authentifizierung erforderlich"}`, http.StatusUnauthorized)
				return
			}

			token, err := jwt.ParseWithClaims(tokenString, &auth.CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unerwarteter Signaturalgorithmus: %v", token.Header["alg"])
				}
				return publicKey, nil
			})

			if err != nil {
				slog.WarnContext(ctx, "Authentifizierung fehlgeschlagen: Token Validierung fehlgeschlagen", slog.Any("error", err))
				http.Error(w, `{"error": "unauthorized", "message": "Ungültiger oder abgelaufener Token"}`, http.StatusUnauthorized)
				return
			}

			if claims, ok := token.Claims.(*auth.CustomClaims); ok && token.Valid {
				if claims.UserID == "" {
					slog.ErrorContext(ctx, "Authentifizierung fehlgeschlagen: UserID fehlt im gültigen Token")
					http.Error(w, `{"error": "internal_error", "message": "Token-Daten unvollständig"}`, http.StatusInternalServerError)
					return
				}
				ctx = context.WithValue(ctx, UserIDContextKey, claims.UserID)
				slog.DebugContext(ctx, "Authentifizierung erfolgreich", slog.String("user_id", claims.UserID))
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				slog.WarnContext(ctx, "Authentifizierung fehlgeschlagen: Ungültige Claims nach erfolgreichem Parsen")
				http.Error(w, `{"error": "unauthorized", "message": "Ungültiger Token"}`, http.StatusUnauthorized)
			}
		})
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization Header fehlt")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", fmt.Errorf("authorization Header Format ist ungültig ('Bearer TOKEN')")
	}

	return parts[1], nil
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(string)
	return userID, ok
}

func GetProjectIDFromContext(ctx context.Context) (string, bool) {
	projectID, ok := ctx.Value(ProjectIDContextKey).(string)
	return projectID, ok
}