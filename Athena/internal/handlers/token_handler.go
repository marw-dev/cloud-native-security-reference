package handlers

import (
	"athena/internal/auth"
	"athena/internal/database"
	"athena/internal/logging"
	"athena/internal/middleware"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Struktur für Token-bezogene Handler
type TokenHandlers struct {
	UserRepo        database.UserRepository
	ProjectRepo 	database.ProjectRepository
	TokenRepo   	database.TokenRepository
	PrivateKey      *rsa.PrivateKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Konstruktor für TokenHandlers
func NewTokenHandlers(repo database.UserRepository, projectRepo database.ProjectRepository,
	tokenRepo database.TokenRepository, pk *rsa.PrivateKey, accessTTL, refreshTTL time.Duration) *TokenHandlers {
	return &TokenHandlers{
		UserRepo:        repo,
		ProjectRepo: projectRepo,
		TokenRepo:   tokenRepo,
		PrivateKey:      pk,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}
}

// RefreshHandler (Verschoben aus auth_handlers.go, Receiver geändert)
func (h *TokenHandlers) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei Refresh", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei Refresh", slog.Any("errors", validationErrs))
		logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
			slog.String("project_id", req.ProjectID),
			slog.String("reason", "validation_failed"),
			slog.Any("validation_errors", validationErrs),
		)
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	oldTokenHash := database.HashToken(req.RefreshToken)

	storedToken, err := h.TokenRepo.GetRefreshTokenByHash(ctx, oldTokenHash)
	if err != nil {
		if errors.Is(err, database.ErrRefreshTokenNotFound) {
			slog.InfoContext(ctx, "Refresh fehlgeschlagen: Alter Token nicht gefunden", slog.String("provided_hash_prefix", oldTokenHash[:8]))
			logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
				slog.String("project_id", req.ProjectID),
				slog.String("reason", "token_not_found"),
			)
			writeJSONError(w, "Ungültiger oder abgelaufener Refresh Token", http.StatusUnauthorized)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen des alten Refresh Tokens", slog.Any("error", err))
			logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
				slog.String("project_id", req.ProjectID),
				slog.String("reason", "db_error_get_token"),
				slog.Any("error", err),
			)
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	ctx = context.WithValue(ctx, middleware.UserIDContextKey, storedToken.UserID.String())

	if storedToken.Revoked || time.Now().UTC().After(storedToken.ExpiresAt) {
		slog.WarnContext(ctx, "Refresh fehlgeschlagen: Alter Token widerrufen oder abgelaufen", slog.String("user_id", storedToken.UserID.String()), slog.Bool("revoked", storedToken.Revoked), slog.Time("expires_at", storedToken.ExpiresAt))
		logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
			slog.String("project_id", req.ProjectID),
			slog.String("reason", "token_revoked_or_expired"),
			slog.Bool("revoked", storedToken.Revoked),
		)
		writeJSONError(w, "Ungültiger oder abgelaufener Refresh Token", http.StatusUnauthorized)
		return
	}

	user, err := h.UserRepo.GetUserByID(ctx, storedToken.UserID)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
			slog.String("project_id", req.ProjectID),
			slog.String("reason", "user_not_found_for_token"),
		)
		handleGetUserError(ctx, w, err, storedToken.UserID.String())
		return
	}

	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, user.ID, req.ProjectID)
	if err != nil {
		if errors.Is(err, database.ErrUserNotInProject) {
			slog.WarnContext(ctx, "Refresh fehlgeschlagen: Benutzer nicht im angeforderten Projekt", slog.String("user_id", user.ID.String()), slog.String("project_id", req.ProjectID))
			logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
				slog.String("project_id", req.ProjectID),
				slog.String("reason", "user_not_in_project"),
			)
			writeJSONError(w, "Ungültiger Refresh Token für dieses Projekt", http.StatusForbidden)
		} else {
			slog.ErrorContext(ctx, "Fehler beim Abrufen der Projektdaten bei Refresh", slog.Any("error", err), slog.String("user_id", user.ID.String()))
			logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
				slog.String("project_id", req.ProjectID),
				slog.String("reason", "db_error_get_project_data"),
				slog.Any("error", err),
			)
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
		return
	}

	err = h.TokenRepo.DeleteRefreshToken(ctx, oldTokenHash)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Löschen des alten Refresh Tokens während der Rotation", slog.Any("error", err), slog.String("user_id", user.ID.String()))
	}

	newRefreshTokenString := auth.GenerateRefreshToken()
	newRefreshTokenHash := database.HashToken(newRefreshTokenString)
	newRefreshExpiresAt := time.Now().UTC().Add(h.RefreshTokenTTL)

	newRefreshTokenDB := &database.RefreshToken{
		TokenHash: newRefreshTokenHash,
		UserID:    user.ID,
		ExpiresAt: newRefreshExpiresAt,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	if err := h.TokenRepo.SaveRefreshToken(ctx, newRefreshTokenDB); err != nil {
		slog.ErrorContext(ctx, "KRITISCH: Fehler beim Speichern des NEUEN Refresh Tokens während der Rotation", slog.Any("error", err), slog.String("user_id", user.ID.String()))
		logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
			slog.String("project_id", req.ProjectID),
			slog.String("reason", "db_error_save_new_token"),
			slog.Any("error", err),
		)
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	newAccessToken, err := auth.GenerateToken(user, projectData.Roles, h.PrivateKey, h.AccessTokenTTL)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Erstellen des neuen Access Tokens bei Refresh", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditFailure,
			slog.String("project_id", req.ProjectID),
			slog.String("reason", "jwt_access_token_generation_failed"),
			slog.Any("error", err),
		)
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	resp := LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenString,
	}
	writeJSONResponse(w, resp, http.StatusOK)
	
	logging.LogAuditEvent(ctx, "AUTH_REFRESH", logging.AuditSuccess,
		slog.String("project_id", req.ProjectID),
	)
	
	slog.InfoContext(ctx, "Access und Refresh Token erfolgreich erneuert (Rotation)", slog.String("user_id", user.ID.String()), slog.String("project_id", req.ProjectID), slog.Bool("roles_included", true))
}