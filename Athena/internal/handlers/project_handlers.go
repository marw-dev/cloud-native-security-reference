package handlers

import (
	"athena/internal/database"
	"athena/internal/logging"
	"athena/internal/middleware"
	"athena/internal/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ProjectHandlers struct {
	ProjectRepo database.ProjectRepository
	RouteRepo   database.RouteRepository
	UserRepo    database.UserRepository
}

// Konstruktor aktualisieren:
func NewProjectHandlers(
	projectRepo database.ProjectRepository,
	routeRepo database.RouteRepository,
	userRepo database.UserRepository,
) *ProjectHandlers {
	return &ProjectHandlers{
		ProjectRepo: projectRepo,
		RouteRepo:   routeRepo,
		UserRepo:    userRepo,
	}
}

func (h *ProjectHandlers) checkProjectAdmin(ctx context.Context, userID uuid.UUID, projectID string) (bool, error) {
	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			return false, nil // Benutzer ist nicht im Projekt, also kein Admin
		}
		return false, err // Echter DB-Fehler
	}

	for _, role := range projectData.Roles {
		if role == "admin" || role == "owner" {
			return true, nil // Benutzer hat die erforderliche Rolle
		}
	}
	return false, nil // Benutzer hat keine Admin-Rolle
}

func (h *ProjectHandlers) checkProjectOwner(ctx context.Context, userID uuid.UUID, projectID string) (bool, error) {
	// 1. Prüfen, ob der Benutzer ein Global-Admin ist
	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err // DB-Fehler
	}
	if user.IsAdmin {
		return true, nil // Global-Admins dürfen alles
	}

	// 2. Wenn kein Global-Admin, prüfen, ob er die "owner"-Rolle im Projekt hat
	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			return false, nil // Benutzer ist nicht im Projekt, also kein Owner
		}
		return false, err // Echter DB-Fehler
	}

	for _, role := range projectData.Roles {
		if role == "owner" {
			return true, nil // Benutzer hat die "owner"-Rolle
		}
	}
	return false, nil // Benutzer hat keine Owner-Rolle
}

// DTO für die Erstellung
type CreateProjectRequest struct {
	Name string `json:"name" validate:"required,min=3"`
}

func (h *ProjectHandlers) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	ownerUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext", slog.Any("error", err), slog.String("user_id_str", userIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}
	user, err := h.UserRepo.GetUserByID(ctx, ownerUserID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr) // Gibt 401/500
		return
	}

	if !user.IsAdmin {
		logging.LogAuditEvent(ctx, "PROJECT_CREATE", logging.AuditFailure,
			slog.String("reason", "permission_denied_not_global_admin"),
		)
		writeJSONError(w, "Zugriff verweigert: Nur globale Administratoren dürfen Projekte erstellen.", http.StatusForbidden)
		return
	}

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// Neues Projekt erstellen
	newProject := models.NewProject(req.Name, ownerUserID)

	// Projekt in der DB speichern
	if err := h.ProjectRepo.CreateProject(ctx, newProject); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern des neuen Projekts", slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	if err := h.ProjectRepo.AddUserToProject(ctx, ownerUserID, newProject.ID.String(), []string{"admin", "owner"}); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Hinzufügen des Projekt-Owners zum Projekt", slog.Any("error", err))
	}

	writeJSONResponse(w, newProject, http.StatusCreated)
	slog.InfoContext(ctx, "Neues Projekt erfolgreich erstellt", slog.String("project_id", newProject.ID.String()), slog.String("user_id", ownerUserID.String()))
}

// Route: PUT /projects/{projectID}/settings
func (h *ProjectHandlers) UpdateProjectSettingsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Benutzer-ID (Admin) aus Kontext holen
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)

	// 2. Projekt-ID aus URL holen
	projectID := chi.URLParam(r, "projectID")
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// 3. Berechtigungsprüfung
	isAdmin, err := h.checkProjectAdmin(ctx, adminUserID, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler bei Admin-Berechtigungsprüfung", slog.Any("error", err), slog.String("user_id", adminUserIDStr), slog.String("project_id", projectID))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}
	if !isAdmin {
		slog.WarnContext(ctx, "Nicht autorisierter Versuch, Projekteinstellungen zu ändern", slog.String("user_id", adminUserIDStr), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "PROJECT_SETTINGS_UPDATE", logging.AuditFailure,
			slog.String("reason", "permission_denied"),
		)
		writeJSONError(w, "Zugriff verweigert: Sie haben keine Administratorrechte für dieses Projekt.", http.StatusForbidden)
		return
	}

	// 4. Request-Body parsen
	var req UpdateProjectSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// 5. Projekt laden und aktualisieren
	projectToUpdate, err := h.ProjectRepo.GetProjectByID(ctx, projectIDUUID)
	if err != nil {
		handleGetUserError(ctx, w, err, projectID) // Gibt 404 zurück, wenn Projekt nicht gefunden
		return
	}

	projectToUpdate.Name = req.Name
	projectToUpdate.Force2FA = *req.Force2FA
	if req.Host != "" {
		projectToUpdate.Host = sql.NullString{String: req.Host, Valid: true}
	} else {
		// Wenn der Benutzer das Feld leert, setzen wir es auf NULL
		projectToUpdate.Host = sql.NullString{Valid: false}
	}

	// 6. In DB speichern
	if err := h.ProjectRepo.UpdateProjectSettings(ctx, projectToUpdate); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern der Projekt-Einstellungen", slog.Any("error", err), slog.String("project_id", projectID))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_SETTINGS_UPDATE", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("new_name", projectToUpdate.Name),
		slog.Bool("force_2fa", projectToUpdate.Force2FA),
	)

	writeJSONResponse(w, projectToUpdate, http.StatusOK)
}

// Route: POST /projects/{projectID}/users
func (h *ProjectHandlers) AddUserToProjectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Benutzer-ID (Admin) aus Kontext holen
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige Admin-UserID im Kontext", slog.Any("error", err), slog.String("user_id_str", adminUserIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	// 2. Projekt-ID aus URL holen
	projectID := chi.URLParam(r, "projectID")
	if _, err := uuid.Parse(projectID); err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// 3. Berechtigungsprüfung
	isOwner, err := h.checkProjectOwner(ctx, adminUserID, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler bei Admin-Berechtigungsprüfung (AddUser)", slog.Any("error", err), slog.String("user_id", adminUserIDStr), slog.String("project_id", projectID))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}
	if !isOwner {
		slog.WarnContext(ctx, "Nicht autorisierter Versuch (Projekt-Admin), Benutzer hinzuzufügen", slog.String("user_id", adminUserIDStr), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "PROJECT_ADD_USER", logging.AuditFailure,
			slog.String("reason", "permission_denied_not_owner"),
		)
		writeJSONError(w, "Zugriff verweigert: Nur Projekt-Owner dürfen Benutzer hinzufügen.", http.StatusForbidden)
		return
	}

	// 4. Request-Body parsen
	var req AddUserToProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// 5. Neuen Benutzer global suchen
	userToAdd, err := h.UserRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			slog.DebugContext(ctx, "Admin versucht, nicht-globalen Benutzer hinzuzufügen", slog.String("email", req.Email), slog.String("project_id", projectID))
			writeJSONError(w, "Benutzer nicht gefunden. Der Benutzer muss global existieren, bevor er zu einem Projekt hinzugefügt werden kann.", http.StatusNotFound)
		} else {
			slog.ErrorContext(ctx, "Fehler bei der Suche nach Benutzer (AddUser)", slog.Any("error", err), slog.String("email", req.Email))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	// 6. Benutzer zum Projekt hinzufügen
	err = h.ProjectRepo.AddUserToProject(ctx, userToAdd.ID, projectID, req.Roles)
	if err != nil {
		slog.WarnContext(ctx, "Fehler beim Hinzufügen des Benutzers zum Projekt (vielleicht schon Mitglied?)", slog.Any("error", err), slog.String("user_id", userToAdd.ID.String()), slog.String("project_id", projectID))
		writeJSONError(w, "Benutzer konnte nicht hinzugefügt werden (möglicherweise bereits Mitglied).", http.StatusConflict)
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_ADD_USER", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("added_user_id", userToAdd.ID.String()),
		slog.String("added_user_email", userToAdd.Email),
		slog.Any("roles", req.Roles),
	)

	slog.InfoContext(ctx, "Benutzer erfolgreich zu Projekt hinzugefügt", slog.String("user_id", userToAdd.ID.String()), slog.String("project_id", projectID))
	w.WriteHeader(http.StatusCreated)
}

// Route: GET /projects
func (h *ProjectHandlers) GetMyProjectsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext", slog.Any("error", err), slog.String("user_id_str", userIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	projects, err := h.ProjectRepo.GetProjectsByUserID(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Projekte für Benutzer", slog.Any("error", err), slog.String("user_id", userIDStr))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, projects, http.StatusOK)
}

// Für PUT /projects/{projectID}/settings
type UpdateProjectSettingsRequest struct {
	Name     string `json:"name" validate:"required,min=3"`
	Force2FA *bool  `json:"force_2fa" validate:"required"`
	Host     string `json:"host"`
}

// Für POST /projects/{projectID}/users
type AddUserToProjectRequest struct {
	Email string   `json:"email" validate:"required,email"`
	Roles []string `json:"roles" validate:"required,min=1"`
}

// Route: POST /projects/{projectID}/routes
func (h *ProjectHandlers) CreateProjectRouteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige Admin-UserID im Kontext", slog.Any("error", err), slog.String("user_id_str", adminUserIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	projectID := chi.URLParam(r, "projectID")
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// Berechtigungsprüfung
	isAdmin, err := h.checkProjectAdmin(ctx, adminUserID, projectID)
	if err != nil || !isAdmin {
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	var req CreateProjectRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// DTO zu Modell konvertieren
	newRoute := models.NewProjectRoute(projectIDUUID, req.Path, req.TargetURL)
	newRoute.RequiredRoles = req.RequiredRoles
	newRoute.CacheTTL = req.CacheTTL
	newRoute.RateLimit = models.RateLimitConfig(req.RateLimit)
	newRoute.CircuitBreaker = models.CircuitBreakerConfig(req.CircuitBreaker)

	if err := h.RouteRepo.CreateProjectRoute(ctx, newRoute); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern der neuen Route", slog.Any("error", err))
		writeJSONError(w, "Fehler beim Speichern (Pfad existiert vielleicht schon)", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, newRoute, http.StatusCreated)
}

// GetProjectRoutesHandler listet alle Routen für ein Projekt auf.
// Route: GET /projects/{projectID}/routes
func (h *ProjectHandlers) GetProjectRoutesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext", slog.Any("error", err), slog.String("user_id_str", userIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	projectID := chi.URLParam(r, "projectID")
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// Berechtigungsprüfung
	_, err = h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}
	
	routes, err := h.RouteRepo.GetProjectRoutes(ctx, projectIDUUID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Abrufen der Routen für UI", slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}
	
	writeJSONResponse(w, routes, http.StatusOK)
}

// Route: GET /projects/{projectID}
func (h *ProjectHandlers) GetProjectDetailsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Benutzer-ID aus Kontext holen
	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	// 2. Projekt-ID aus URL holen
	projectID := chi.URLParam(r, "projectID")
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// 3. Berechtigungsprüfung
	_, err = h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		slog.WarnContext(ctx, "Nicht autorisierter Versuch, Projektdetails abzurufen", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "PROJECT_GET_DETAILS", logging.AuditFailure,
			slog.String("reason", "permission_denied"),
		)
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// 4. Projektdaten laden
	project, err := h.ProjectRepo.GetProjectByID(ctx, projectIDUUID)
	if err != nil {
		handleGetUserError(ctx, w, err, projectID)
		return
	}

	// 5. Erfolgreich
	writeJSONResponse(w, project, http.StatusOK)
}

// Route: GET /projects/{projectID}/users
func (h *ProjectHandlers) GetProjectUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)
	projectID := chi.URLParam(r, "projectID")
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID in URL", http.StatusBadRequest)
		return
	}

	// Berechtigungsprüfung
	isAdmin, err := h.checkProjectAdmin(ctx, adminUserID, projectID)
	if err != nil || !isAdmin {
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// Lade die Benutzer
	dbUsers, err := h.ProjectRepo.GetUsersByProjectID(ctx, projectIDUUID)
	if err != nil {
		slog.ErrorContext(ctx, "DB-Fehler beim Laden der Projekt-Benutzer", slog.Any("error", err), slog.String("project_id", projectID))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	// Konvertiere DB-Modell in Response-DTO
	responseUsers := make([]ProjectUserResponse, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		var roles []string
		if dbUser.RolesString.Valid && dbUser.RolesString.String != "" {
			roles = strings.Split(dbUser.RolesString.String, ",")
		} else {
			roles = []string{}
		}
		
		responseUsers = append(responseUsers, ProjectUserResponse{
			ID:        uuid.MustParse(dbUser.IDString),
			Email:     dbUser.Email,
			IsAdmin:   dbUser.IsAdmin,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
			Roles:     roles,
		})
	}

	writeJSONResponse(w, responseUsers, http.StatusOK)
}

// Route: PUT /projects/{projectID}/users/{userID}
func (h *ProjectHandlers) UpdateUserRolesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Admin-Prüfung
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)
	projectID := chi.URLParam(r, "projectID")
	
	isOwner, err := h.checkProjectOwner(ctx, adminUserID, projectID)
	if err != nil || !isOwner {
		logging.LogAuditEvent(ctx, "PROJECT_USER_UPDATE_ROLES", logging.AuditFailure, slog.String("reason", "permission_denied"))
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// 2. Ziel-UserID holen
	targetUserIDStr := chi.URLParam(r, "userID")
	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		writeJSONError(w, "Ungültige Ziel-UserID in URL", http.StatusBadRequest)
		return
	}

	// 3. Request-Body (Rollen) parsen
	var req UpdateUserRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// 4. In DB speichern (KORREKTUR: 'projectID' (string) übergeben, nicht 'projectIDUUID')
	err = h.ProjectRepo.UpdateUserRoles(ctx, targetUserID, projectID, req.Roles)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			writeJSONError(w, "Benutzer nicht in diesem Projekt gefunden", http.StatusNotFound)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Aktualisieren der Benutzerrollen", slog.Any("error", err), slog.String("user_id", targetUserIDStr))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_USER_UPDATE_ROLES", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("target_user_id", targetUserIDStr),
		slog.Any("new_roles", req.Roles),
	)

	w.WriteHeader(http.StatusOK)
}

// Route: DELETE /projects/{projectID}/users/{userID}
func (h *ProjectHandlers) RemoveUserFromProjectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Admin-Prüfung
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)
	projectID := chi.URLParam(r, "projectID")
	
	isOwner, err := h.checkProjectOwner(ctx, adminUserID, projectID)
	if err != nil || !isOwner {
		logging.LogAuditEvent(ctx, "PROJECT_USER_REMOVE", logging.AuditFailure, slog.String("reason", "permission_denied"))
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// 2. Ziel-UserID holen
	targetUserIDStr := chi.URLParam(r, "userID")
	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		writeJSONError(w, "Ungültige Ziel-UserID in URL", http.StatusBadRequest)
		return
	}

	// 3. (Optional) Verhindern, dass sich der Admin selbst entfernt
	if adminUserID == targetUserID {
		writeJSONError(w, "Sie können sich nicht selbst aus einem Projekt entfernen.", http.StatusBadRequest)
		return
	}

	// 4. Aus DB löschen (KORREKTUR: 'projectID' (string) übergeben, nicht 'projectIDUUID')
	err = h.ProjectRepo.RemoveUserFromProject(ctx, targetUserID, projectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			writeJSONError(w, "Benutzer nicht in diesem Projekt gefunden", http.StatusNotFound)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Entfernen des Benutzers", slog.Any("error", err), slog.String("user_id", targetUserIDStr))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_USER_REMOVE", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("removed_user_id", targetUserIDStr),
	)

	w.WriteHeader(http.StatusNoContent) // 204
}

// Route: DELETE /projects/{projectID}/routes/{routeID}
func (h *ProjectHandlers) DeleteProjectRouteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Admin-Prüfung
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)
	projectID := chi.URLParam(r, "projectID")
	
	isAdmin, err := h.checkProjectAdmin(ctx, adminUserID, projectID)
	if err != nil || !isAdmin {
		slog.WarnContext(ctx, "Nicht autorisierter Versuch, Route zu löschen", 
			slog.String("user_id", adminUserIDStr), 
			slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "PROJECT_ROUTE_DELETE", logging.AuditFailure,
			slog.String("reason", "permission_denied"),
		)
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// 2. Route-ID holen
	routeIDStr := chi.URLParam(r, "routeID")
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		writeJSONError(w, "Ungültige Routen-ID in URL", http.StatusBadRequest)
		return
	}

	// 3. Route aus DB löschen
	if err := h.RouteRepo.DeleteProjectRoute(ctx, routeID); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Löschen der Projekt-Route", slog.Any("error", err), slog.String("route_id", routeIDStr))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_ROUTE_DELETE", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("route_id", routeIDStr),
	)

	w.WriteHeader(http.StatusNoContent)
}

// Route: PUT /projects/{projectID}/routes/{routeID}
func (h *ProjectHandlers) UpdateProjectRouteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Admin-Prüfung
	adminUserIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	adminUserID, _ := uuid.Parse(adminUserIDStr)
	projectID := chi.URLParam(r, "projectID")
	
	isAdmin, err := h.checkProjectAdmin(ctx, adminUserID, projectID)
	if err != nil || !isAdmin {
		logging.LogAuditEvent(ctx, "PROJECT_ROUTE_UPDATE", logging.AuditFailure,
			slog.String("reason", "permission_denied"),
		)
		writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
		return
	}

	// 2. Route-ID holen
	routeIDStr := chi.URLParam(r, "routeID")
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		writeJSONError(w, "Ungültige Routen-ID in URL", http.StatusBadRequest)
		return
	}

	// 3. Request-Body parsen
	var req UpdateProjectRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}
	
	// 4. Bestehende Route laden
	routeToUpdate, err := h.RouteRepo.GetProjectRouteByID(ctx, routeID)
	if err != nil {
		handleGetUserError(ctx, w, err, routeIDStr) // 404
		return
	}

	// 5. Felder aktualisieren
	routeToUpdate.Path = req.Path
	routeToUpdate.TargetURL = req.TargetURL
	routeToUpdate.RequiredRoles = req.RequiredRoles
	routeToUpdate.CacheTTL = req.CacheTTL
	routeToUpdate.RateLimit = models.RateLimitConfig(req.RateLimit)
	routeToUpdate.CircuitBreaker = models.CircuitBreakerConfig(req.CircuitBreaker)

	// 6. In DB speichern
	if err := h.RouteRepo.UpdateProjectRoute(ctx, routeToUpdate); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren der Projekt-Route", slog.Any("error", err), slog.String("route_id", routeIDStr))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	logging.LogAuditEvent(ctx, "PROJECT_ROUTE_UPDATE", logging.AuditSuccess,
		slog.String("project_id", projectID),
		slog.String("route_id", routeIDStr),
	)

	writeJSONResponse(w, routeToUpdate, http.StatusOK)
}