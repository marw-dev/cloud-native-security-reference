package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	IsAdmin      bool      `db:"is_admin"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`

	GlobalOTPEnabled bool           `db:"global_otp_enabled"`
	GlobalOTPSecret  sql.NullString `db:"global_otp_secret"`
	GlobalOTPAuthURL sql.NullString `db:"global_otp_auth_url"`
}

func NewUser(email string, passwordHash string) *User {
	now := time.Now().UTC()
	return &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		IsAdmin:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
		GlobalOTPEnabled: false,
	}
}