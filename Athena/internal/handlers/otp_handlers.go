package handlers

import (
	"athena/internal/auth"
	"athena/internal/database"
	"athena/internal/logging"
	"athena/internal/middleware"
	"athena/internal/models"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

type OTPHandlers struct {
	UserRepo    database.UserRepository
	ProjectRepo database.ProjectRepository
	TokenRepo   database.TokenRepository
	IssuerName string
	PrivateKey      *rsa.PrivateKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewOTPHandlers(repo database.UserRepository, projectRepo database.ProjectRepository,
	tokenRepo database.TokenRepository, issuerName string, pk *rsa.PrivateKey, accessTTL, refreshTTL time.Duration) *OTPHandlers {
	return &OTPHandlers{
		UserRepo:        repo,
		ProjectRepo: projectRepo,
		TokenRepo:   tokenRepo,
		IssuerName:  issuerName,
		PrivateKey:      pk,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}
}

func (h *OTPHandlers) SetupOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		slog.ErrorContext(ctx, "UserID nicht gefunden in context für OTP Setup")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (SetupOTPHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusInternalServerError)
		return
	}

	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}

	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      h.IssuerName,
		AccountName: user.Email,
	})
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Generieren des OTP-Secrets", slog.Any("error", err), slog.String("user_id", userIDStr))
		
		
		logging.LogAuditEvent(ctx, "OTP_SETUP", logging.AuditFailure,
			slog.String("reason", "otp_key_generation_failed"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	otpSecretBase32 := otpKey.Secret()
	otpAuthURL := auth.GenerateOTPAuthURL(otpKey)

	err = h.ProjectRepo.SetOTPSecretAndURL(ctx, userID, projectID, otpSecretBase32, otpAuthURL)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern des OTP-Secrets/URL in DB", slog.Any("error", err), slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_SETUP", logging.AuditFailure,
			slog.String("reason", "db_save_secret_failed"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	qrCodeBytes, err := auth.GenerateQRCodePNG(otpAuthURL)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Generieren des QR-Codes", slog.Any("error", err), slog.String("user_id", userIDStr))
		
		
		logging.LogAuditEvent(ctx, "OTP_SETUP", logging.AuditFailure,
			slog.String("reason", "qr_code_generation_failed"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	resp := OTPSetupResponse{
		Secret:   otpSecretBase32,
		QRCode:   base64.StdEncoding.EncodeToString(qrCodeBytes),
		AuthURL:  otpAuthURL,
	}

	writeJSONResponse(w, resp, http.StatusOK)
	
	
	logging.LogAuditEvent(ctx, "OTP_SETUP", logging.AuditSuccess)
	
	slog.InfoContext(ctx, "OTP-Setup initiiert", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
}

func (h *OTPHandlers) VerifyOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		slog.ErrorContext(ctx, "UserID nicht gefunden in context für OTP Verify")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (SetupOTPHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusInternalServerError)
		return
	}

	var req OTPSecureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei OTP Verify", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei OTP Verify", slog.Any("errors", validationErrs))
		
		
		logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditFailure,
			slog.String("reason", "validation_failed"),
			slog.Any("validation_errors", validationErrs),
		)
		
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	otpSecretDB, _, err := h.ProjectRepo.GetUserOTPSecretAndStatus(ctx, userID, projectID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	if !otpSecretDB.Valid || otpSecretDB.String == "" {
		slog.WarnContext(ctx, "OTP Verify Versuch ohne vorheriges Setup", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditFailure,
			slog.String("reason", "otp_not_initialized"),
		)
		
		writeJSONError(w, "OTP Setup wurde nicht korrekt initialisiert", http.StatusBadRequest)
		return
	}

	valid, err := auth.ValidateOTPCode(otpSecretDB.String, req.OTPCode)
	if err != nil {
		slog.ErrorContext(ctx, "Interner Fehler bei OTP-Validierung", slog.Any("error", err), slog.String("user_id", userIDStr))
		
		
		logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditFailure,
			slog.String("reason", "internal_otp_validation_error"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler bei OTP-Prüfung", http.StatusInternalServerError)
		return
	}
	if !valid {
		slog.InfoContext(ctx, "Ungültiger OTP-Code bei Verify eingegeben", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditFailure,
			slog.String("reason", "invalid_otp_code"),
		)
		
		writeJSONError(w, "Ungültiger OTP-Code", http.StatusUnauthorized)
		return
	}

	err = h.ProjectRepo.EnableOTP(ctx, userID, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktivieren von OTP in DB", slog.Any("error", err), slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditFailure,
			slog.String("reason", "db_enable_otp_failed"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}

	projectData, err := h.ProjectRepo.GetUserProjectData(ctx, userID, projectID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}

	h.issueTokensAndRespond(ctx, w, user, projectData.Roles)
	
	logging.LogAuditEvent(ctx, "OTP_VERIFY", logging.AuditSuccess)
	
	slog.InfoContext(ctx, "OTP erfolgreich verifiziert und aktiviert", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
	w.WriteHeader(http.StatusOK)
}

func (h *OTPHandlers) DisableOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userIDStr == "" {
		slog.ErrorContext(ctx, "UserID nicht gefunden in context für OTP Disable")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (SetupOTPHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusInternalServerError)
		return
	}

	var req OTPSecureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei OTP Disable", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei OTP Disable", slog.Any("errors", validationErrs))
		
		
		logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditFailure,
			slog.String("reason", "validation_failed"),
			slog.Any("validation_errors", validationErrs),
		)
		
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	otpSecretDB, isEnabled, err := h.ProjectRepo.GetUserOTPSecretAndStatus(ctx, userID, projectID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	if !isEnabled || !otpSecretDB.Valid || otpSecretDB.String == "" {
		slog.WarnContext(ctx, "Versuch, OTP zu deaktivieren, obwohl es nicht aktiv ist", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditFailure,
			slog.String("reason", "otp_not_enabled"),
		)
		
		writeJSONError(w, "OTP ist für diesen Benutzer in diesem Projekt nicht aktiviert", http.StatusBadRequest)
		return
	}

	valid, err := auth.ValidateOTPCode(otpSecretDB.String, req.OTPCode)
	if err != nil {
		slog.ErrorContext(ctx, "Interner Fehler bei OTP-Validierung für Disable", slog.Any("error", err), slog.String("user_id", userIDStr))
		
		
		logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditFailure,
			slog.String("reason", "internal_otp_validation_error"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler bei OTP-Prüfung", http.StatusInternalServerError)
		return
	}
	if !valid {
		slog.InfoContext(ctx, "Ungültiger Bestätigungs-OTP-Code bei Disable eingegeben", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditFailure,
			slog.String("reason", "invalid_otp_code_for_confirmation"),
		)
		
		writeJSONError(w, "Ungültiger OTP-Code zur Bestätigung", http.StatusUnauthorized)
		return
	}

	err = h.ProjectRepo.DisableOTP(ctx, userID, projectID)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Deaktivieren von OTP in DB", slog.Any("error", err), slog.String("user_id", userIDStr), slog.String("project_id", projectID))
		
		
		logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditFailure,
			slog.String("reason", "db_disable_otp_failed"),
			slog.Any("error", err),
		)
		
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	
	logging.LogAuditEvent(ctx, "OTP_DISABLE", logging.AuditSuccess)
	
	slog.InfoContext(ctx, "OTP erfolgreich deaktiviert", slog.String("user_id", userIDStr), slog.String("project_id", projectID))
	w.WriteHeader(http.StatusOK)
}


func (h *OTPHandlers) issueTokensAndRespond(ctx context.Context, w http.ResponseWriter, user *models.User, roles []string) {
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
	}
	writeJSONResponse(w, resp, http.StatusOK)
}

// LoginOTPHandler (Verschoben aus auth_handlers.go, Receiver geändert)
func (h *OTPHandlers) LoginOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginOTPRequest

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (LoginOTPHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusBadRequest)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei OTP Login", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei OTP Login", slog.Any("errors", validationErrs))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "validation_failed"),
			slog.Any("validation_errors", validationErrs),
		)
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	user, projectData, err := h.ProjectRepo.GetUserAndProjectDataByEmail(ctx, req.Email, projectID)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "user_not_found_or_not_in_project"),
		)
		handleGetUserError(ctx, w, err, req.Email)
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		slog.InfoContext(ctx, "OTP Login fehlgeschlagen: Falsches Passwort", slog.String("email", req.Email))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "invalid_password"),
		)
		writeJSONError(w, "Ungültige E-Mail, Passwort oder OTP-Code", http.StatusUnauthorized)
		return
	}

	if !projectData.OTPEnabled {
		slog.WarnContext(ctx, "OTP Login Versuch für Benutzer ohne aktiviertes OTP", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "otp_not_enabled"),
		)
		writeJSONError(w, "OTP ist für diesen Account in diesem Projekt nicht aktiviert", http.StatusBadRequest)
		return
	}

	if !projectData.OTPSecret.Valid || projectData.OTPSecret.String == "" {
		slog.ErrorContext(ctx, "OTP Login fehlgeschlagen: OTP ist aktiv, aber kein Secret gespeichert", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "internal_otp_secret_missing"),
		)
		writeJSONError(w, "Interner Konfigurationsfehler", http.StatusInternalServerError)
		return
	}

	valid, err := auth.ValidateOTPCode(projectData.OTPSecret.String, req.OTPCode)
	if err != nil {
		slog.ErrorContext(ctx, "Interner Fehler bei OTP-Validierung (Login)", slog.Any("error", err), slog.String("user_id", user.ID.String()))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "internal_otp_validation_error"),
			slog.Any("error", err),
		)
		writeJSONError(w, "Interner Serverfehler bei OTP-Prüfung", http.StatusInternalServerError)
		return
	}
	if !valid {
		slog.InfoContext(ctx, "Ungültiger OTP-Code bei Login eingegeben", slog.String("user_id", user.ID.String()))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "invalid_otp_code"),
		)
		writeJSONError(w, "Ungültige E-Mail, Passwort oder OTP-Code", http.StatusUnauthorized)
		return
	}

	h.issueTokensAndRespond(ctx, w, user, projectData.Roles)
	logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP", logging.AuditSuccess,
		slog.String("email", req.Email),
		slog.String("user_id", user.ID.String()),
		slog.String("result", "tokens_issued"),
	)
	slog.InfoContext(ctx, "Benutzer erfolgreich eingeloggt (mit OTP)", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
}

// LoginOTPStandaloneHandler (Verschoben aus auth_handlers.go, Receiver geändert)
func (h *OTPHandlers) LoginOTPStandaloneHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginOTPStandaloneRequest

	projectID, ok := middleware.GetProjectIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "ProjectID nicht im Kontext gefunden (LoginOTPStandaloneHandler)")
		writeJSONError(w, "Interner Serverfehler (ProjectID fehlt)", http.StatusBadRequest)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.WarnContext(ctx, "Ungültiger Request Body bei OTP-Standalone Login", slog.Any("error", err))
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if validationErrs := validateRequest(ctx, req); validationErrs != nil {
		slog.WarnContext(ctx, "Validierungsfehler bei OTP-Standalone Login", slog.Any("errors", validationErrs))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "validation_failed"),
			slog.Any("validation_errors", validationErrs),
		)
		writeJSONResponse(w, validationErrs, http.StatusBadRequest)
		return
	}

	user, projectData, err := h.ProjectRepo.GetUserAndProjectDataByEmail(ctx, req.Email, projectID)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("reason", "user_not_found_or_not_in_project"),
		)
		handleGetUserError(ctx, w, err, req.Email)
		return
	}

	if !projectData.OTPEnabled {
		slog.WarnContext(ctx, "OTP-Standalone Login Versuch für Benutzer ohne aktiviertes OTP", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "otp_not_enabled"),
		)
		writeJSONError(w, "OTP ist für diesen Account in diesem Projekt nicht aktiviert", http.StatusBadRequest)
		return
	}

	if !projectData.OTPSecret.Valid || projectData.OTPSecret.String == "" {
		slog.ErrorContext(ctx, "OTP-Standalone Login fehlgeschlagen: OTP ist aktiv, aber kein Secret gespeichert", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "internal_otp_secret_missing"),
		)
		writeJSONError(w, "Interner Konfigurationsfehler", http.StatusInternalServerError)
		return
	}

	valid, err := auth.ValidateOTPCode(projectData.OTPSecret.String, req.OTPCode)
	if err != nil {
		slog.ErrorContext(ctx, "Interner Fehler bei OTP-Validierung (Standalone Login)", slog.Any("error", err), slog.String("user_id", user.ID.String()))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "internal_otp_validation_error"),
			slog.Any("error", err),
		)
		writeJSONError(w, "Interner Serverfehler bei OTP-Prüfung", http.StatusInternalServerError)
		return
	}
	if !valid {
		slog.InfoContext(ctx, "Ungültiger OTP-Code bei Standalone Login eingegeben", slog.String("user_id", user.ID.String()))
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditFailure,
			slog.String("email", req.Email),
			slog.String("user_id", user.ID.String()),
			slog.String("reason", "invalid_otp_code"),
		)
		writeJSONError(w, "Ungültige E-Mail oder OTP-Code", http.StatusUnauthorized)
		return
	}

	h.issueTokensAndRespond(ctx, w, user, projectData.Roles)
	logging.LogAuditEvent(ctx, "AUTH_LOGIN_OTP_STANDALONE", logging.AuditSuccess,
		slog.String("email", req.Email),
		slog.String("user_id", user.ID.String()),
		slog.String("result", "tokens_issued"),
	)
	slog.InfoContext(ctx, "Benutzer erfolgreich eingeloggt (via OTP-Standalone)", slog.String("user_id", user.ID.String()), slog.String("project_id", projectID))
}