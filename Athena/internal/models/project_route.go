package models

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)


type RateLimitConfig struct {
	Limit  int    `json:"limit" db:"rate_limit_limit"`
	Window string `json:"window" db:"rate_limit_window"`
}

type CircuitBreakerConfig struct {
	FailureThreshold int    `json:"failure_threshold" db:"cb_threshold"`
	OpenTimeout      string `json:"open_timeout" db:"cb_timeout"`
}

type ProjectRoute struct {
	ID            uuid.UUID            `json:"id" db:"id"`
	ProjectID     uuid.UUID            `json:"project_id" db:"project_id"`
	Path          string               `json:"path" db:"path"`
	TargetURL     string               `json:"target_url" db:"target_url"`
	RequiredRoles []string             `json:"required_roles"`
	RolesString   sql.NullString       `json:"-" db:"required_roles"`
	CacheTTL      string               `json:"cache_ttl" db:"cache_ttl"`
	RateLimit     RateLimitConfig      `json:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func (pr *ProjectRoute) AfterGet() {
	if pr.RolesString.Valid && pr.RolesString.String != "" {
		pr.RequiredRoles = strings.Split(pr.RolesString.String, ",")
	} else {
		pr.RequiredRoles = []string{}
	}
}

func (pr *ProjectRoute) BeforeSave() {
	if len(pr.RequiredRoles) > 0 {
		pr.RolesString = sql.NullString{
			String: strings.Join(pr.RequiredRoles, ","),
			Valid:  true,
		}
	} else {
		pr.RolesString = sql.NullString{String: "", Valid: false}
	}
}

func NewProjectRoute(projectID uuid.UUID, path, targetURL string) *ProjectRoute {
	now := time.Now().UTC()
	return &ProjectRoute{
		ID:            uuid.New(),
		ProjectID:     projectID,
		Path:          path,
		TargetURL:     targetURL,
		RequiredRoles: []string{},
		RateLimit:     RateLimitConfig{Limit: 0, Window: "0s"},
		CircuitBreaker: CircuitBreakerConfig{FailureThreshold: 0, OpenTimeout: "0s"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}