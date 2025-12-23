package models

import (
	"database/sql"

	"github.com/google/uuid"
)


type UserProjectData struct {
	UserID     uuid.UUID      `db:"user_id"`
	ProjectID  string         `db:"project_id"`
	Roles      []string       `db:"roles"`
	OTPEnabled bool           `db:"otp_enabled"`
	OTPSecret  sql.NullString `db:"otp_secret"`
	OTPAuthURL sql.NullString `db:"otp_auth_url"`
}