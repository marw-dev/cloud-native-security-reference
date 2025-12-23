package handlers

import (
	"athena/internal/database"
	"athena/internal/middleware"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type UserHandlers struct {
	UserRepo    database.UserRepository
	ProjectRepo database.ProjectRepository
}

func NewUserHandlers(userRepo database.UserRepository, projectRepo database.ProjectRepository) *UserHandlers {
	return &UserHandlers{
		UserRepo:    userRepo,
		ProjectRepo: projectRepo,
	}
}


func (h *UserHandlers) GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		slog.ErrorContext(ctx, "UserID not found in context for authenticated route")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (GetCurrentUserHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusInternalServerError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext gefunden", slog.Any("error", err), slog.String("user_id_str", userIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation im Token", http.StatusInternalServerError)
		return
	}

	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			slog.WarnContext(ctx, "Benutzer aus Token nicht in DB gefunden", slog.String("user_id", userIDStr))
			writeJSONError(w, "Benutzer nicht gefunden", http.StatusNotFound)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen des Benutzers nach ID", slog.Any("error", err), slog.String("user_id", userIDStr))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			slog.WarnContext(ctx, "Benutzer ist nicht Teil des angefragten Projekts", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
			writeJSONError(w, "Benutzer nicht im Projekt gefunden", http.StatusForbidden) // 403, da der Benutzer existiert, aber keinen Zugriff hat
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen der Projektdaten", slog.Any("error", err), slog.String("user_id", userIDStr), slog.String("project_id", projectID))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	response := ProfileResponse{
		ID:         user.ID,
		Email:      user.Email,
		Roles:      projectData.Roles,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
		OTPEnabled: projectData.OTPEnabled,
	}

	writeJSONResponse(w, response, http.StatusOK)
	slog.DebugContext(ctx, "Benutzerprofil erfolgreich abgerufen", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
}

func (h *UserHandlers) UpdateCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		slog.ErrorContext(ctx, "UserID nicht gefunden in context für Update")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext für Update", slog.Any("error", err), slog.String("user_id_str", userIDStr))
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusInternalServerError)
		return
	}

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (GetCurrentUserHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusInternalServerError)
		return
	}


	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei Profil-Update", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei Profil-Update", slog.Any("errors", validationErrs))
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	currentUser, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			slog.WarnContext(ctx, "Benutzer aus Token nicht in DB gefunden für Update", slog.String("user_id", userIDStr))
			writeJSONError(w, "Benutzer nicht gefunden", http.StatusNotFound)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen des Benutzers für Update", slog.Any("error", err), slog.String("user_id", userIDStr))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			slog.WarnContext(ctx, "Benutzer (Update) ist nicht Teil des angefragten Projekts", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
			writeJSONError(w, "Benutzer nicht im Projekt gefunden", http.StatusForbidden)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen der Projektdaten (Update)", slog.Any("error", err), slog.String("user_id", userIDStr), slog.String("project_id", projectID))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	currentUser.Email = req.Email
	if err := h.UserRepo.UpdateUser(ctx, currentUser); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren des Benutzers in DB", slog.Any("error", err), slog.String("user_id", userIDStr))
		writeJSONError(w, "Interner Serverfehler beim Speichern", http.StatusInternalServerError)
		return
	}

	response := ProfileResponse{
		ID:        currentUser.ID,
		Email:     currentUser.Email,
		Roles:     projectData.Roles,
		CreatedAt: currentUser.CreatedAt,
		UpdatedAt: currentUser.UpdatedAt,
		OTPEnabled: projectData.OTPEnabled,
	}
	writeJSONResponse(w, response, http.StatusOK)
	slog.InfoContext(ctx, "Benutzerprofil erfolgreich aktualisiert", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
}