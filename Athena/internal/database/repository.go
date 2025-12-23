package database

import (
	"athena/internal/models"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// --- Fehler (Bleiben global) ---
var ErrUserNotFound = errors.New("benutzer nicht gefunden")
var ErrRefreshTokenNotFound = errors.New("refresh token nicht gefunden oder ungültig")
var ErrUserNotInProject = errors.New("benutzer ist diesem projekt nicht zugeordnet")

// --- DTOs (Bleiben hier) ---
type RefreshToken struct {
	TokenHash string    `db:"token_hash"`
	UserID    uuid.UUID `db:"user_id"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
	CreatedAt time.Time `db:"created_at"`
}

// DBPinger ist ein Interface, das von *sqlx.DB und unserem Repo implementiert wird
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// UserRepository kümmert sich NUR um globale Benutzer
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	GetUserCount(ctx context.Context) (int, error)

	SetGlobalOTPSecretAndURL(ctx context.Context, userID uuid.UUID, secret, authURL string) error
	EnableGlobalOTP(ctx context.Context, userID uuid.UUID) error
	DisableGlobalOTP(ctx context.Context, userID uuid.UUID) error
	GetGlobalOTPSecretAndStatus(ctx context.Context, userID uuid.UUID) (secret sql.NullString, isEnabled bool, err error)
}

// TokenRepository kümmert sich NUR um Refresh Tokens
type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, rt *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
}

type ProjectUserDB struct {
	IDString    string    `db:"id"`
	Email       string    `db:"email"`
	IsAdmin     bool      `db:"is_admin"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	RolesString sql.NullString `db:"roles"`
}

// ProjectRepository kümmert sich um Projekte und deren Benutzer-Beziehungen
type ProjectRepository interface {
	// Projekt-Management
	CreateProject(ctx context.Context, project *models.Project) error
	GetProjectByID(ctx context.Context, projectID uuid.UUID) (*models.Project, error)
	GetProjectsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Project, error)
	UpdateProjectSettings(ctx context.Context, project *models.Project) error
	GetProjectContextMap(ctx context.Context) (map[string]string, error)

	// Benutzer-Projekt-Beziehungen
	AddUserToProject(ctx context.Context, userID uuid.UUID, projectID string, roles []string) error
	GetUserProjectData(ctx context.Context, userID uuid.UUID, projectID string) (*models.UserProjectData, error)
	// GetUserAndProjectDataByEmail ist komplexer, da es beide Repos benötigt.
	// Wir implementieren es, indem wir die DB-Logik aufteilen.
	GetUserAndProjectDataByEmail(ctx context.Context, email string, projectID string) (*models.User, *models.UserProjectData, error)
	GetUsersByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ProjectUserDB, error)
	UpdateUserRoles(ctx context.Context, userID uuid.UUID, projectID string, roles []string) error
    RemoveUserFromProject(ctx context.Context, userID uuid.UUID, projectID string) error

	// OTP (ist an die Benutzer-Projekt-Beziehung gebunden)
	SetOTPSecretAndURL(ctx context.Context, userID uuid.UUID, projectID string, secret, authURL string) error
	EnableOTP(ctx context.Context, userID uuid.UUID, projectID string) error
	DisableOTP(ctx context.Context, userID uuid.UUID, projectID string) error
	GetUserOTPSecretAndStatus(ctx context.Context, userID uuid.UUID, projectID string) (secret sql.NullString, isEnabled bool, err error)
}


// RouteRepository kümmert sich NUR um Projekt-Routen
type RouteRepository interface {
	CreateProjectRoute(ctx context.Context, route *models.ProjectRoute) error
	GetProjectRouteByID(ctx context.Context, routeID uuid.UUID) (*models.ProjectRoute, error)
	GetProjectRoutes(ctx context.Context, projectID uuid.UUID) ([]*models.ProjectRoute, error)
	UpdateProjectRoute(ctx context.Context, route *models.ProjectRoute) error
	DeleteProjectRoute(ctx context.Context, routeID uuid.UUID) error
	GetAllProjectRoutes(ctx context.Context) ([]*models.ProjectRoute, error)
}