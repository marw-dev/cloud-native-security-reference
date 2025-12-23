package database

import (
	"athena/internal/models"
	"context"
)

// sqlxRepository ist die konkrete Implementierung, die die DB-Verbindung hält.
// Sie wird alle vier Interfaces (User, Project, Route, Token) implementieren.
type sqlxRepository struct {
	db *DB
}

// Stellen Sie sicher, dass die Struct die Interfaces implementiert
var _ UserRepository = (*sqlxRepository)(nil)
var _ ProjectRepository = (*sqlxRepository)(nil)
var _ RouteRepository = (*sqlxRepository)(nil)
var _ TokenRepository = (*sqlxRepository)(nil)
var _ DBPinger = (*sqlxRepository)(nil)

// --- Konstruktoren ---

func NewUserRepository(db *DB) UserRepository {
	return &sqlxRepository{db: db}
}

func NewProjectRepository(db *DB) ProjectRepository {
	return &sqlxRepository{db: db}
}

func NewRouteRepository(db *DB) RouteRepository {
	return &sqlxRepository{db: db}
}

func NewTokenRepository(db *DB) TokenRepository {
	return &sqlxRepository{db: db}
}

// PingContext implementiert DBPinger
func (r *sqlxRepository) PingContext(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// --- Komplexe Multi-Repo-Implementierung ---

// GetUserAndProjectDataByEmail benötigt Zugriff auf User- und Projektdaten.
// Da sqlxRepository beides implementiert, können wir die Methoden hier wiederverwenden.
func (r *sqlxRepository) GetUserAndProjectDataByEmail(ctx context.Context, email string, projectID string) (*models.User, *models.UserProjectData, error) {
	// 1. Globalen Benutzer holen (ruft die Implementierung in user_repository.go auf)
	user, err := r.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, err // Enthält ErrUserNotFound, falls nicht gefunden
	}

	// 2. Projektdaten holen (ruft die Implementierung in project_repository.go auf)
	projectData, err := r.GetUserProjectData(ctx, user.ID, projectID)
	if err != nil {
		// Benutzer existiert, ist aber nicht im Projekt ODER anderer DB-Fehler
		return user, nil, err // Enthält ErrUserNotInProject
	}

	// 3. Beides gefunden
	return user, projectData, nil
}