package handlers

import (
	"athena/internal/auth"
	"athena/internal/config"
	"athena/internal/database"
	"athena/internal/logging"
	"athena/internal/middleware"
	"athena/internal/models"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type AuthHandlers struct {
	UserRepo    database.UserRepository
	ProjectRepo database.ProjectRepository
	TokenRepo   database.TokenRepository
	PrivateKey  *rsa.PrivateKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Config      *config.Config
}

func NewAuthHandlers(
	userRepo database.UserRepository,
	projectRepo database.ProjectRepository,
	tokenRepo database.TokenRepository,
	pk *rsa.PrivateKey,
	accessTTL, refreshTTL time.Duration,
	cfg *config.Config,
) *AuthHandlers {
	return &AuthHandlers{
		UserRepo:    userRepo,
		ProjectRepo: projectRepo,
		TokenRepo:   tokenRepo,
		PrivateKey:  pk,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		Config:      cfg,
	}
}

// RegisterHandler prüft jetzt den Context auf eine Projekt-ID
func (h *AuthHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req RegisterRequest // {Email, Password, RegistrationSecret}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	existingUser, err := h.UserRepo.GetUserByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, database.ErrUserNotFound) {
		slog.ErrorContext(ctx, "Fehler beim Prüfen des Benutzers", slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}
	if existingUser != nil {
		logging.LogAuditEvent(ctx, "AUTH_REGISTER", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "email_already_exists"),
		)
		writeJSONError(w, "Ein Benutzer mit dieser E-Mail existiert bereits", http.StatusConflict)
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	newUser := models.NewUser(req.Email, hashedPassword)

	// PRÜFE, ob Aegis eine Projekt-ID injiziert hat
	projectID, isProjectRegistration := middleware.GetProjectIDFromContext(ctx)

	if isProjectRegistration {
		// --- FALL 1: KUNDEN-REGISTRIERUNG (z.B. von webshop.com) ---
		// Das Admin-Secret wird ignoriert.
		newUser.IsAdmin = false
		slog.InfoContext(ctx, "Neue Kunden-Registrierung", "email", req.Email, "projectID", projectID)

		if err := h.UserRepo.CreateUser(ctx, newUser); err != nil {
			slog.ErrorContext(ctx, "Fehler beim Speichern des Kunden-Benutzers", slog.Any("error", err))
			writeJSONError(w, "Interner Serverfehler beim Speichern", http.StatusInternalServerError)
			return
		}

		// Benutzer SOFORT dem Projekt zuweisen (z.B. mit der Rolle "user")
		err = h.ProjectRepo.AddUserToProject(ctx, newUser.ID, projectID, []string{"user"})
		if err != nil {
			slog.ErrorContext(ctx, "Kunde erstellt, aber Projekt-Zuweisung fehlgeschlagen", "err", err, "user_ID", newUser.ID, "project_id", projectID)
			// In einer echten Transaktion würden wir den Benutzer jetzt zurückrollen
			writeJSONError(w, "Interner Serverfehler bei Projektzuweisung", http.StatusInternalServerError)
			return
		}
		logging.LogAuditEvent(ctx, "AUTH_REGISTER_CUSTOMER", logging.AuditSuccess,
			slog.String("email", req.Email),
			slog.String("user_id", newUser.ID.String()),
			slog.String("project_id", projectID),
		)

	} else {
		// --- FALL 2: ADMIN-REGISTRIERUNG (z.B. von athena.deine-firma.de) ---
		userCount, err := h.UserRepo.GetUserCount(ctx)
		if err != nil {
			writeJSONError(w, "Interner Serverfehler (DB-Zählung)", http.StatusInternalServerError)
			return
		}

		if userCount == 0 {
			// Der allererste Benutzer ist immer Admin
			newUser.IsAdmin = true
			slog.InfoContext(ctx, "Erster Benutzer wird als Admin registriert", "email", req.Email)
		} else {
			// Prüfe, ob ein Admin-Secret erforderlich ist UND ob es übereinstimmt
			if h.Config.RegistrationSecret != "" && req.RegistrationSecret == h.Config.RegistrationSecret {
				newUser.IsAdmin = true
				slog.InfoContext(ctx, "Neuer Admin-Benutzer wird registriert (via Secret)", "email", req.Email)
			} else {
				// Normale Registrierung (Kunde) oder falsches Secret
				newUser.IsAdmin = false
				slog.InfoContext(ctx, "Neuer Standardbenutzer (Kunde) wird registriert (ohne Projekt)", "email", req.Email)
			}
		}
		if err := h.UserRepo.CreateUser(ctx, newUser); err != nil {
			slog.ErrorContext(ctx, "Fehler beim Speichern des Admin/Standard-Benutzers", slog.Any("error", err))
			writeJSONError(w, "Interner Serverfehler beim Speichern", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// LoginHandler (Zurückgesetzt auf die saubere, Header-basierte Logik)
func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Prüfe, ob Aegis (basierend auf der Domain) eine Projekt-ID injiziert hat
	projectID, ok := middleware.GetProjectIDFromContext(ctx)

	if ok && projectID != "" {
		// Fall B: Projekt-Login (Kunde ODER Projekt-Admin)
		h.handleProjectLogin(w, r, projectID)
	} else {
		// Fall A: Globaler Login (Nur für Admins der AthenaUI)
		h.handleGlobalLogin(w, r)
	}
}

// handleGlobalLogin (Die ursprüngliche, saubere Funktion)
func (h *AuthHandlers) handleGlobalLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	user, err := h.UserRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_GLOBAL", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "user_not_found"),
		)
		handleGetUserError(ctx, w, err, req.Email) // Gibt 401
		return
	}
	ctx = context.WithValue(ctx, middleware.UserIDContextKey, user.ID.String())

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_GLOBAL", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "invalid_password"),
		)
		writeJSONError(w, "Ungültige E-Mail oder Passwort", http.StatusUnauthorized)
		return
	}

	if user.IsAdmin {
		// FALL A: Der Benutzer ist ein Global-Admin
		if user.GlobalOTPEnabled {
			slog.InfoContext(ctx, "Globaler Admin-Login erfordert 2FA", slog.String("user_id", user.ID.String()))
			logging.LogAuditEvent(ctx, "AUTH_LOGIN_GLOBAL", logging.AuditSuccess,
				slog.String("result", "global_otp_required"),
			)
			writeJSONResponse(w, map[string]bool{"global_otp_required": true}, http.StatusOK)
			return
		}

		// Globaler Admin-Token (ohne Projekt-ID)
		slog.InfoContext(ctx, "Global-Admin erfolgreich eingeloggt (ohne 2FA)", slog.String("user_id", user.ID.String()))
		h.issueTokensAndRespond(ctx, w, user, nil, "") // Leere projectID
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_GLOBAL", logging.AuditSuccess,
			slog.String("result", "tokens_issued_admin"),
		)
		return

	} else {
		// FALL B: Der Benutzer ist ein normaler Benutzer (z.B. Projekt-Admin)
		// Er darf sich einloggen, erhält aber keine Admin-Rechte im Token.
		// 2FA wird hier nicht geprüft, da 2FA Projekt-bezogen ist.
		
		slog.InfoContext(ctx, "Normaler Benutzer (Projekt-Admin/User) erfolgreich eingeloggt", slog.String("user_id", user.ID.String()))
		
		// WICHTIG: Token enthält `is_admin: false` und KEINE Rollen.
		// Die Rollen werden erst bei Projekt-API-Aufrufen über die `X-Project-ID` relevant.
		h.issueTokensAndRespond(ctx, w, user, nil, "") // Leere projectID
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_GLOBAL", logging.AuditSuccess,
			slog.String("result", "tokens_issued_user"),
		)
		return
	}

}

// handleProjectLogin (Die ursprüngliche, saubere Funktion, inkl. Grace-Token)
func (h *AuthHandlers) handleProjectLogin(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	// 1. Benutzer UND Projektdaten abrufen
	user, projectData, err := h.ProjectRepo.GetUserAndProjectDataByEmail(ctx, req.Email, projectID)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_PROJECT", logging.AuditFailure,
			slog.String("email", req.Email), slog.String("project_id", projectID),
			slog.String("reason", "user_not_found_or_not_in_project"),
		)
		handleGetUserError(ctx, w, err, req.Email) // 401
		return
	}
	ctx = context.WithValue(ctx, middleware.UserIDContextKey, user.ID.String())

	// 2. Projekt-Einstellungen (2FA-Zwang) abrufen
	projectIDUUID, err := uuid.Parse(projectID)
	if err != nil {
		writeJSONError(w, "Ungültige Projekt-ID", http.StatusBadRequest)
		return
	}
	projectSettings, err := h.ProjectRepo.GetProjectByID(ctx, projectIDUUID)
	if err != nil {
		handleGetUserError(ctx, w, err, projectID)
		return
	}

	// 3. Passwort prüfen
	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_PROJECT", logging.AuditFailure,
			slog.String("email", req.Email), slog.String("project_id", projectID),
			slog.String("reason", "invalid_password"),
		)
		writeJSONError(w, "Ungültige E-Mail oder Passwort", http.StatusUnauthorized)
		return
	}

	// 4. 2FA-Zwang-Prüfung
	if projectSettings.Force2FA && !projectData.OTPEnabled {
		slog.WarnContext(ctx, "2FA-Zwang erkannt. Stelle Grace-Token aus.", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_PROJECT", logging.AuditSuccess,
			slog.String("reason", "project_forces_2fa_not_enabled"),
			slog.String("result", "issuing_grace_token"),
		)

		graceToken, err := auth.GenerateToken(user, projectData.Roles, h.PrivateKey, h.Config.JWTGraceTokenTTL)
		if err != nil {
			slog.ErrorContext(ctx, "Fehler beim Erstellen des Grace Tokens", slog.String("user_id", user.ID.String()), slog.Any("error", err))
			writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
			return
		}

		// Sende Grace-Token UND Projekt-ID
		writeJSONResponse(w, map[string]interface{}{
			"grace_token":              graceToken,
			"force_2fa_setup_required": true,
			"project_id":               projectID, // Wichtig für die UI
		}, http.StatusOK)
		return
	}

	// 5. 2FA-Aktiv-Prüfung
	if projectData.OTPEnabled {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_PROJECT", logging.AuditSuccess,
			slog.String("result", "otp_required"),
		)
		writeJSONResponse(w, map[string]interface{}{
			"otp_required": true,
			"project_id":   projectID, // Wichtig für die UI
		}, http.StatusOK)
		return
	}

	// 6. Erfolgreicher Projekt-Login
	logging.LogAuditEvent(ctx, "AUTH_LOGIN_PROJECT", logging.AuditSuccess,
		slog.String("result", "tokens_issued"),
	)
	h.issueTokensAndRespond(ctx, w, user, projectData.Roles, projectID)
}

// issueTokensAndRespond (WICHTIG: Fügt project_id zur Antwort hinzu)
func (h *AuthHandlers) issueTokensAndRespond(ctx context.Context, w http.ResponseWriter, user *models.User, roles []string, projectID string) {
	if roles == nil {
		roles = []string{}
	}

	accessToken, err := auth.GenerateToken(user, roles, h.PrivateKey, h.AccessTokenTTL)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Erstellen des Access JWT für Login", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	refreshTokenString := auth.GenerateRefreshToken()
	refreshTokenHash := database.HashToken(refreshTokenString)
	refreshExpiresAt := time.Now().UTC().Add(h.RefreshTokenTTL)

	refreshTokenDB := &database.RefreshToken{
		TokenHash: refreshTokenHash,
		UserID:    user.ID,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	if err := h.TokenRepo.SaveRefreshToken(ctx, refreshTokenDB); err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern des Refresh Tokens", slog.String("user_id", user.ID.String()), slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ProjectID:    projectID, // <-- HINZUGEFÜGT
	}
	writeJSONResponse(w, resp, http.StatusOK)
}