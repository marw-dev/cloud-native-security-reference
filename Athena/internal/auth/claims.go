package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	IsAdmin bool    `json:"is_admin"`
	jwt.RegisteredClaims
}
