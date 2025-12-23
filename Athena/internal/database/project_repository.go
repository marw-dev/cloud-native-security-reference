package database

import (
	"athena/internal/models"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AddUserToProject fügt einen Benutzer zu einem Projekt hinzu
func (r *sqlxRepository) AddUserToProject(ctx context.Context, userID uuid.UUID, projectID string, roles []string) error {
	rolesString := strings.Join(roles, ",")
	query := `INSERT INTO user_projects (user_id, project_id, roles, otp_enabled)
			   VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		userID.String(),
		projectID,
		rolesString,
		false,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Hinzufügen des Benutzers zum Projekt", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}
	slog.DebugContext(ctx, "Benutzer erfolgreich zu Projekt hinzugefügt", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}

// GetUserProjectData holt projektspezifische Daten
func (r *sqlxRepository) GetUserProjectData(ctx context.Context, userID uuid.UUID, projectID string) (*models.UserProjectData, error) {
	var dbData struct {
		models.UserProjectData
		RolesString  sql.NullString `db:"roles"`
		UserIDString string         `db:"user_id"`
	}
	query := `SELECT user_id, project_id, roles, otp_enabled, otp_secret, otp_auth_url
	           FROM user_projects WHERE user_id = ? AND project_id = ? LIMIT 1`

	err := r.db.GetContext(ctx, &dbData, query, userID.String(), projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Benutzer nicht im Projekt gefunden", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
			return nil, ErrUserNotInProject // Wichtig: Eigener Fehler
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Projektdaten", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return nil, err
	}

	data := dbData.UserProjectData
	parsedID, parseErr := uuid.Parse(dbData.UserIDString)
	if parseErr != nil {
		slog.ErrorContext(ctx, "Fehler beim Parsen der User-ID aus user_projects", slog.Any("error", parseErr), slog.String("id_string", dbData.UserIDString))
		return nil, parseErr
	}
	data.UserID = parsedID

	if dbData.RolesString.Valid && dbData.RolesString.String != "" {
		data.Roles = strings.Split(dbData.RolesString.String, ",")
	} else {
		data.Roles = []string{}
	}
	slog.DebugContext(ctx, "Projektdaten für Benutzer erfolgreich gefunden", slog.String("user_id", data.UserID.String()))
	return &data, nil
}

func (r *sqlxRepository) CreateProject(ctx context.Context, project *models.Project) error {
	query := `INSERT INTO projects (id, name, owner_user_id, force_2fa, created_at, updated_at)
			   VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		project.ID.String(),
		project.Name,
		project.OwnerUserID.String(),
		project.Force2FA,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Erstellen des Projekts", slog.Any("error", err), slog.String("name", project.Name))
		return err
	}
	slog.DebugContext(ctx, "Projekt erfolgreich in DB erstellt", slog.String("project_id", project.ID.String()))
	return nil
}

func (r *sqlxRepository) GetProjectByID(ctx context.Context, projectID uuid.UUID) (*models.Project, error) {
	var project models.Project
	var dbProject struct {
		models.Project
		IDString          string `db:"id"`
		OwnerUserIDString string `db:"owner_user_id"`
	}

	query := `SELECT * FROM projects WHERE id = ? LIMIT 1`
	err := r.db.GetContext(ctx, &dbProject, query, projectID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Projekt nicht gefunden", slog.String("project_id", projectID.String()))
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des Projekts nach ID", slog.Any("error", err), slog.String("project_id", projectID.String()))
		return nil, err
	}

	project = dbProject.Project
	project.ID, _ = uuid.Parse(dbProject.IDString)
	project.OwnerUserID, _ = uuid.Parse(dbProject.OwnerUserIDString)

	return &project, nil
}
func (r *sqlxRepository) GetProjectsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Project, error) {
	query := `SELECT p.* FROM projects p
	           JOIN user_projects up ON p.id = up.project_id
	           WHERE up.user_id = ?`

	var dbProjects []struct {
		models.Project
		IDString          string `db:"id"`
		OwnerUserIDString string `db:"owner_user_id"`
	}

	err := r.db.SelectContext(ctx, &dbProjects, query, userID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Projekte für Benutzer", slog.Any("error", err), slog.String("user_id", userID.String()))
		return nil, err
	}

	projects := make([]*models.Project, 0, len(dbProjects))
	for _, dbp := range dbProjects {
		project := dbp.Project
		project.ID, _ = uuid.Parse(dbp.IDString)
		project.OwnerUserID, _ = uuid.Parse(dbp.OwnerUserIDString)
		projects = append(projects, &project)
	}

	slog.DebugContext(ctx, "Projekte für Benutzer erfolgreich abgerufen", slog.String("user_id", userID.String()), slog.Int("count", len(projects)))
	return projects, nil
}

func (r *sqlxRepository) UpdateProjectSettings(ctx context.Context, project *models.Project) error {
	query := `UPDATE projects SET name = ?, force_2fa = ?, host = ?, updated_at = ?
	           WHERE id = ?`

	now := time.Now().UTC()
	project.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, query,
		project.Name,
		project.Force2FA,
		project.Host,
		project.UpdatedAt,
		project.ID.String(),
	)

	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren der Projekt-Einstellungen", slog.Any("error", err), slog.String("project_id", project.ID.String()))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, nicht existierendes Projekt zu aktualisieren", slog.String("project_id", project.ID.String()))
		return ErrUserNotFound
	}

	slog.DebugContext(ctx, "Projekt-Einstellungen erfolgreich aktualisiert", slog.String("project_id", project.ID.String()))
	return nil
}

// GetUsersByProjectID (KORRIGIERT: Gibt jetzt []*ProjectUserDB zurück)
func (r *sqlxRepository) GetUsersByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ProjectUserDB, error) {
	var dbUsers []*ProjectUserDB
    
    // KORRIGIERTE QUERY: Holt 'up.roles'
	query := `SELECT u.id, u.email, u.is_admin, u.created_at, u.updated_at, up.roles 
              FROM users u
              JOIN user_projects up ON u.id = up.user_id
              WHERE up.project_id = ?`

	err := r.db.SelectContext(ctx, &dbUsers, query, projectID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Benutzer für Projekt", slog.Any("error", err), slog.String("project_id", projectID.String()))
		return nil, err
	}

	return dbUsers, nil
}

// UpdateUserRoles (NEUE FUNKTION)
func (r *sqlxRepository) UpdateUserRoles(ctx context.Context, userID uuid.UUID, projectID string, roles []string) error {
	rolesString := strings.Join(roles, ",")
	query := `UPDATE user_projects SET roles = ? WHERE user_id = ? AND project_id = ?`

	result, err := r.db.ExecContext(ctx, query, rolesString, userID.String(), projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren der Benutzerrollen", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotInProject // Benutzer war nicht im Projekt
	}
	
	slog.DebugContext(ctx, "Benutzerrollen erfolgreich aktualisiert", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}

// RemoveUserFromProject (NEUE FUNKTION)
func (r *sqlxRepository) RemoveUserFromProject(ctx context.Context, userID uuid.UUID, projectID string) error {
	query := `DELETE FROM user_projects WHERE user_id = ? AND project_id = ?`
	
	result, err := r.db.ExecContext(ctx, query, userID.String(), projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Entfernen des Benutzers aus Projekt", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotInProject // Benutzer war nicht im Projekt
	}

	slog.DebugContext(ctx, "Benutzer erfolgreich aus Projekt entfernt", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}


// --- OTP-Methoden ---

func (r *sqlxRepository) SetOTPSecretAndURL(ctx context.Context, userID uuid.UUID, projectID string, secret, authURL string) error {
	query := `UPDATE user_projects SET otp_secret = ?, otp_auth_url = ?
	           WHERE user_id = ? AND project_id = ?`

	result, err := r.db.ExecContext(ctx, query, secret, authURL, userID.String(), projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Setzen des OTP-Secrets/URL", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, OTP-Secret für nicht existierenden Benutzer/Projekt zu setzen", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return ErrUserNotInProject
	}
	slog.DebugContext(ctx, "OTP-Secret und URL erfolgreich gesetzt", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}

func (r *sqlxRepository) EnableOTP(ctx context.Context, userID uuid.UUID, projectID string) error {
	query := `UPDATE user_projects SET otp_enabled = TRUE
	           WHERE user_id = ? AND project_id = ?`

	result, err := r.db.ExecContext(ctx, query, userID.String(), projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktivieren von OTP", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, OTP für nicht existierenden Benutzer/Projekt zu aktivieren", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return ErrUserNotInProject
	}
	slog.InfoContext(ctx, "OTP erfolgreich aktiviert", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}

func (r *sqlxRepository) DisableOTP(ctx context.Context, userID uuid.UUID, projectID string) error {
	query := `UPDATE user_projects SET otp_enabled = FALSE, otp_secret = NULL, otp_auth_url = NULL
	           WHERE user_id = ? AND project_id = ?`

	result, err := r.db.ExecContext(ctx, query, userID.String(), projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Deaktivieren von OTP", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, OTP für nicht existierenden Benutzer/Projekt zu deaktivieren", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return ErrUserNotInProject
	}
	slog.InfoContext(ctx, "OTP erfolgreich deaktiviert", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
	return nil
}

func (r *sqlxRepository) GetUserOTPSecretAndStatus(ctx context.Context, userID uuid.UUID, projectID string) (secret sql.NullString, isEnabled bool, err error) {
	query := `SELECT otp_secret, otp_enabled FROM user_projects
	           WHERE user_id = ? AND project_id = ? LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, userID.String(), projectID)
	err = row.Scan(&secret, &isEnabled)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Benutzer für OTP-Statusabfrage nicht im Projekt gefunden", slog.String("user_id", userID.String()), slog.String("project_id", projectID))
			return secret, false, ErrUserNotInProject
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des OTP-Status/Secrets", slog.Any("error", err), slog.String("user_id", userID.String()), slog.String("project_id", projectID))
		return secret, false, err
	}

	slog.DebugContext(ctx, "OTP-Status/Secret erfolgreich abgerufen", slog.String("user_id", userID.String()), slog.String("project_id", projectID), slog.Bool("enabled", isEnabled))
	return secret, isEnabled, nil
}

func (r *sqlxRepository) GetProjectContextMap(ctx context.Context) (map[string]string, error) {
	var results []struct {
		Host      string `db:"host"`
		ProjectID string `db:"id"`
	}
	query := `SELECT id, host FROM projects WHERE host IS NOT NULL AND host != ''`

	err := r.db.SelectContext(ctx, &results, query)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Projekt-Context-Map", slog.Any("error", err))
		return nil, err
	}

	contextMap := make(map[string]string)
	for _, res := range results {
		contextMap[res.Host] = res.ProjectID
	}

	slog.DebugContext(ctx, "Projekt-Context-Map erfolgreich geladen", slog.Int("count", len(contextMap)))
	return contextMap, nil
}