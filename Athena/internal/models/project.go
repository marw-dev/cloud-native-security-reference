package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID         uuid.UUID `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	OwnerUserID uuid.UUID `db:"owner_user_id" json:"-"`
	Force2FA   bool      `db:"force_2fa" json:"force_2fa"`
	Host       sql.NullString `db:"host" json:"host,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

func NewProject(name string, ownerUserID uuid.UUID) *Project {
	now := time.Now().UTC()
	return &Project{
		ID: 			uuid.New(), // X-Project-ID
		Name:			name,
		OwnerUserID: 	ownerUserID,
		Force2FA: 		false,
		CreatedAt: 		now,
		UpdatedAt: 		now,
	}
}