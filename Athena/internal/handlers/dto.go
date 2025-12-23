package handlers

import (
	"time"

	"github.com/google/uuid"
)

// RegisterRequest für Registrierung. (Feld ist optional)
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	RegistrationSecret string `json:"registration_secret,omitempty"` // Kunde lässt das leer
}

// LoginRequest für Login. (Einfach)
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginOTPRequest für den 2FA-Login. (Benötigt jetzt Projekt-ID)
type LoginOTPRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	OTPCode  string `json:"otp_code" validate:"required,numeric,len=6"`
	ProjectID string `json:"project_id,omitempty"` // Für Projekt-OTP
}

// LoginResponse
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ProjectID    string `json:"project_id,omitempty"` // Teilt der UI den Kontext mit
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	ProjectID    string `json:"project_id" validate:"required"`
}

type UpdateProfileRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ProfileResponse struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Roles      []string  `json:"roles"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	OTPEnabled bool      `json:"otp_enabled"`
}

type OTPSecureRequest struct {
	OTPCode  string `json:"otp_code" validate:"required,numeric,len=6"`
}

type OTPSetupResponse struct {
	Secret   string `json:"secret"`
	QRCode   string `json:"qr_code"` // Base64 PNG
	AuthURL  string `json:"auth_url"`
}

type LoginOTPStandaloneRequest struct {
	Email   string `json:"email" validate:"required,email"`
	OTPCode string `json:"otp_code" validate:"required,numeric,len=6"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type RouteRateLimitConfig struct {
	Limit  int    `json:"limit"`
	Window string `json:"window" validate:"duration"`
}

type RouteCircuitBreakerConfig struct {
	FailureThreshold int    `json:"failure_threshold"`
	OpenTimeout      string `json:"open_timeout" validate:"duration"`
}

type RouteRequestBase struct {
	Path           string                    `json:"path" validate:"required,startswith=/"`
	TargetURL      string                    `json:"target_url" validate:"required,url"`
	RequiredRoles  []string                  `json:"required_roles"`
	CacheTTL       string                    `json:"cache_ttl" validate:"duration"`
	RateLimit      RouteRateLimitConfig      `json:"rate_limit"`
	CircuitBreaker RouteCircuitBreakerConfig `json:"circuit_breaker"`
}

// POST /projects/{projectID}/routes
type CreateProjectRouteRequest struct {
	RouteRequestBase
}

// PUT /projects/{projectID}/routes/{routeID}
type UpdateProjectRouteRequest struct {
	RouteRequestBase
}

type UpdateUserRolesRequest struct {
	Roles []string `json:"roles" validate:"required"`
}

type ProjectUserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Roles     []string  `json:"roles"`
}