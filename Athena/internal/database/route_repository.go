package database

import (
	"athena/internal/models"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// (dbProjectRoute-Struktur und ToModel-Helfer gehören hierher)
type dbProjectRoute struct {
	ID                 string         `db:"id"`
	ProjectID          string         `db:"project_id"`
	Path               string         `db:"path"`
	TargetURL          string         `db:"target_url"`
	RolesString        sql.NullString `db:"required_roles"`
	CacheTTL           string         `db:"cache_ttl"`
	RateLimitLimit     int            `db:"rate_limit_limit"`
	RateLimitWindow    string         `db:"rate_limit_window"`
	CbThreshold        int            `db:"cb_threshold"`
	CbTimeout          string         `db:"cb_timeout"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`
}

func (dbpr *dbProjectRoute) ToModel() *models.ProjectRoute {
	pr := &models.ProjectRoute{
		ID:          uuid.MustParse(dbpr.ID),
		ProjectID:   uuid.MustParse(dbpr.ProjectID),
		Path:        dbpr.Path,
		TargetURL:   dbpr.TargetURL,
		RolesString: dbpr.RolesString,
		CacheTTL:    dbpr.CacheTTL,
		RateLimit: models.RateLimitConfig{
			Limit:  dbpr.RateLimitLimit,
			Window: dbpr.RateLimitWindow,
		},
		CircuitBreaker: models.CircuitBreakerConfig{
			FailureThreshold: dbpr.CbThreshold,
			OpenTimeout:      dbpr.CbTimeout,
		},
		CreatedAt: dbpr.CreatedAt,
		UpdatedAt: dbpr.UpdatedAt,
	}
	pr.AfterGet()
	return pr
}

func (r *sqlxRepository) CreateProjectRoute(ctx context.Context, route *models.ProjectRoute) error {
	route.BeforeSave() // Konvertiert RequiredRoles -> RolesString
	query := `INSERT INTO project_routes (id, project_id, path, target_url, 
	                      required_roles, cache_ttl, 
	                      rate_limit_limit, rate_limit_window, 
	                      cb_threshold, cb_timeout, 
	                      created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		route.ID.String(), route.ProjectID.String(), route.Path, route.TargetURL,
		route.RolesString, route.CacheTTL,
		route.RateLimit.Limit, route.RateLimit.Window,
		route.CircuitBreaker.FailureThreshold, route.CircuitBreaker.OpenTimeout,
		route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Erstellen der Projekt-Route", slog.Any("error", err), slog.String("path", route.Path))
		return err
	}
	return nil
}

func (r *sqlxRepository) GetProjectRouteByID(ctx context.Context, routeID uuid.UUID) (*models.ProjectRoute, error) {
	var dbRoute dbProjectRoute
	query := `SELECT * FROM project_routes WHERE id = ? LIMIT 1`
	err := r.db.GetContext(ctx, &dbRoute, query, routeID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound // Oder ErrRouteNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Route nach ID", slog.Any("error", err), slog.String("route_id", routeID.String()))
		return nil, err
	}
	return dbRoute.ToModel(), nil
}

func (r *sqlxRepository) GetProjectRoutes(ctx context.Context, projectID uuid.UUID) ([]*models.ProjectRoute, error) {
	var dbRoutes []dbProjectRoute
	query := `SELECT * FROM project_routes WHERE project_id = ?`
	err := r.db.SelectContext(ctx, &dbRoutes, query, projectID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Routen für Projekt", slog.Any("error", err), slog.String("project_id", projectID.String()))
		return nil, err
	}

	routes := make([]*models.ProjectRoute, 0, len(dbRoutes))
	for _, dbRoute := range dbRoutes {
		routes = append(routes, dbRoute.ToModel())
	}
	return routes, nil
}

func (r *sqlxRepository) UpdateProjectRoute(ctx context.Context, route *models.ProjectRoute) error {
	route.BeforeSave()
	route.UpdatedAt = time.Now().UTC()
	query := `UPDATE project_routes SET 
	            path = ?, target_url = ?, required_roles = ?, cache_ttl = ?,
	            rate_limit_limit = ?, rate_limit_window = ?,
	            cb_threshold = ?, cb_timeout = ?,
	            updated_at = ?
	          WHERE id = ? AND project_id = ?`
	_, err := r.db.ExecContext(ctx, query,
		route.Path, route.TargetURL, route.RolesString, route.CacheTTL,
		route.RateLimit.Limit, route.RateLimit.Window,
		route.CircuitBreaker.FailureThreshold, route.CircuitBreaker.OpenTimeout,
		route.UpdatedAt,
		route.ID.String(), route.ProjectID.String(),
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren der Projekt-Route", slog.Any("error", err), slog.String("route_id", route.ID.String()))
		return err
	}
	return nil
}

func (r *sqlxRepository) DeleteProjectRoute(ctx context.Context, routeID uuid.UUID) error {
	query := `DELETE FROM project_routes WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, routeID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Löschen der Projekt-Route", slog.Any("error", err), slog.String("route_id", routeID.String()))
		return err
	}
	return nil
}

func (r *sqlxRepository) GetAllProjectRoutes(ctx context.Context) ([]*models.ProjectRoute, error) {
	var dbRoutes []dbProjectRoute
	query := `SELECT * FROM project_routes`
	err := r.db.SelectContext(ctx, &dbRoutes, query)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen ALLER Routen für Aegis", slog.Any("error", err))
		return nil, err
	}

	routes := make([]*models.ProjectRoute, 0, len(dbRoutes))
	for _, dbRoute := range dbRoutes {
		routes = append(routes, dbRoute.ToModel())
	}
	return routes, nil
}