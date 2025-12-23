package auth

import (
	"athena/internal/models"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func GenerateToken(user *models.User, roles []string, privateKey *rsa.PrivateKey, ttl time.Duration) (string, error) {
	if user == nil {
		return "", fmt.Errorf("benutzer darf nicht nil sein")
	}
	if privateKey == nil {
		return "", fmt.Errorf("privater Schlüssel darf nicht nil sein")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	claims := CustomClaims{
		UserID:  user.ID.String(),
		Roles:   roles,
		IsAdmin: user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "athena-service",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		slog.Error("Fehler beim Signieren des JWT", slog.Any("error", err))
		return "", fmt.Errorf("fehler beim Signieren des JWT: %w", err)
	}

	slog.Debug("JWT erfolgreich erstellt", slog.String("user_id", user.ID.String()), slog.Time("expires_at", expiresAt))
	return signedToken, nil
}

func GenerateRefreshToken() string {
	return uuid.NewString()
}