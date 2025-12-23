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

type adminContextKey string
const isAdminKey adminContextKey = "isAdmin"

// AdminHandlers verwaltet Admin-spezifische Aktionen wie globales OTP
type AdminHandlers struct {
	UserRepo        database.UserRepository
	TokenRepo       database.TokenRepository // Für LoginOTP
	IssuerName      string
	PrivateKey      *rsa.PrivateKey
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewAdminHandlers(
	userRepo database.UserRepository,
	tokenRepo database.TokenRepository,
	issuerName string,
	pk *rsa.PrivateKey,
	accessTTL, refreshTTL time.Duration,
) *AdminHandlers {
	return &AdminHandlers{
		UserRepo:        userRepo,
		TokenRepo:       tokenRepo,
		IssuerName:      issuerName,
		PrivateKey:      pk,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}
}

func (h *AdminHandlers) OnlyAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userIDStr, ok := middleware.GetUserIDFromContext(ctx)
		if !ok {
			writeJSONError(w, "Authentifizierung erforderlich", http.StatusUnauthorized)
			return
		}
		userID, _ := uuid.Parse(userIDStr)

		user, err := h.UserRepo.GetUserByID(ctx, userID)
		if err != nil || !user.IsAdmin {
			logging.LogAuditEvent(ctx, "ADMIN_ACTION_DENIED", logging.AuditFailure, slog.String("reason", "not_an_admin"))
			writeJSONError(w, "Zugriff verweigert", http.StatusForbidden)
			return
		}

		ctx = context.WithValue(ctx, isAdminKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetupGlobalOTPHandler
func (h *AdminHandlers) SetupGlobalOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Robuste Prüfung des Benutzers, der aus dem Auth-Middleware-Kontext kommt
	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "UserID nicht gefunden (Admin Setup)")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext (Admin Setup)", "error", err)
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}
	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}

	otpKey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      h.IssuerName + " (Admin)",
		AccountName: user.Email,
	})
	if err != nil {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_SETUP", logging.AuditFailure, slog.String("reason", "key_generation_failed"))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	otpSecretBase32 := otpKey.Secret()
	otpAuthURL := auth.GenerateOTPAuthURL(otpKey)

	err = h.UserRepo.SetGlobalOTPSecretAndURL(ctx, userID, otpSecretBase32, otpAuthURL)
	if err != nil {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_SETUP", logging.AuditFailure, slog.String("reason", "db_save_failed"))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	qrCodeBytes, err := auth.GenerateQRCodePNG(otpAuthURL)
	if err != nil {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_SETUP", logging.AuditFailure, slog.String("reason", "qr_code_failed"))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	resp := OTPSetupResponse{
		Secret:   otpSecretBase32,
		QRCode:   base64.StdEncoding.EncodeToString(qrCodeBytes),
		AuthURL:  otpAuthURL,
	}
	writeJSONResponse(w, resp, http.StatusOK)
	logging.LogAuditEvent(ctx, "ADMIN_OTP_SETUP", logging.AuditSuccess)
}

// VerifyGlobalOTPHandler
func (h *AdminHandlers) VerifyGlobalOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "UserID nicht gefunden (Admin Verify)")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext (Admin Verify)", "error", err)
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	var req OTPSecureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	otpSecretDB, _, err := h.UserRepo.GetGlobalOTPSecretAndStatus(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	if !otpSecretDB.Valid || otpSecretDB.String == "" {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_VERIFY", logging.AuditFailure, slog.String("reason", "otp_not_initialized"))
		writeJSONError(w, "OTP Setup wurde nicht korrekt initialisiert", http.StatusBadRequest)
		return
	}

	valid, err := auth.ValidateOTPCode(otpSecretDB.String, req.OTPCode)
	if err != nil || !valid {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_VERIFY", logging.AuditFailure, slog.String("reason", "invalid_otp_code"))
		writeJSONError(w, "Ungültiger OTP-Code", http.StatusUnauthorized)
		return
	}

	err = h.UserRepo.EnableGlobalOTP(ctx, userID)
	if err != nil {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_VERIFY", logging.AuditFailure, slog.String("reason", "db_enable_failed"))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	logging.LogAuditEvent(ctx, "ADMIN_OTP_VERIFY", logging.AuditSuccess)

	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	h.issueTokensAndRespond(ctx, w, user, nil)
}

// DisableGlobalOTPHandler
func (h *AdminHandlers) DisableGlobalOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "UserID nicht gefunden (Admin Disable)")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext (Admin Disable)", "error", err)
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}

	var req OTPSecureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	otpSecretDB, isEnabled, err := h.UserRepo.GetGlobalOTPSecretAndStatus(ctx, userID) // <-- userID ist jetzt sicher
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	if !isEnabled {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_DISABLE", logging.AuditFailure, slog.String("reason", "otp_not_enabled"))
		writeJSONError(w, "Globales OTP ist nicht aktiviert", http.StatusBadRequest)
		return
	}

	valid, err := auth.ValidateOTPCode(otpSecretDB.String, req.OTPCode)
	if err != nil || !valid {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_DISABLE", logging.AuditFailure, slog.String("reason", "invalid_otp_code"))
		writeJSONError(w, "Ungültiger OTP-Code", http.StatusUnauthorized)
		return
	}

	err = h.UserRepo.DisableGlobalOTP(ctx, userID)
	if err != nil {
		logging.LogAuditEvent(ctx, "ADMIN_OTP_DISABLE", logging.AuditFailure, slog.String("reason", "db_disable_failed"))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	logging.LogAuditEvent(ctx, "ADMIN_OTP_DISABLE", logging.AuditSuccess)
	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}
	// Admins erhalten globale Tokens (keine Rollen, da 'is_admin' im Token ist)
	h.issueTokensAndRespond(ctx, w, user, nil)
}

// LoginGlobalOTPHandler
func (h *AdminHandlers) LoginGlobalOTPHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req LoginOTPRequest // { Email, Password, OTPCode }

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Ungültiger JSON Body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	user, err := h.UserRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditFailure, slog.String("email", req.Email), slog.String("reason", "user_not_found"))
		handleGetUserError(ctx, w, err, req.Email) // Gibt 401
		return
	}
	ctx = context.WithValue(ctx, middleware.UserIDContextKey, user.ID.String())

	if !user.IsAdmin {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditFailure, slog.String("reason", "not_an_admin"))
		writeJSONError(w, "Ungültige Anmeldedaten", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditFailure, slog.String("reason", "invalid_password"))
		writeJSONError(w, "Ungültige Anmeldedaten", http.StatusUnauthorized)
		return
	}

	if !user.GlobalOTPEnabled || !user.GlobalOTPSecret.Valid {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditFailure, slog.String("reason", "global_otp_not_enabled"))
		writeJSONError(w, "Ungültige Anmeldedaten", http.StatusUnauthorized)
		return
	}

	valid, err := auth.ValidateOTPCode(user.GlobalOTPSecret.String, req.OTPCode)
	if err != nil || !valid {
		logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditFailure, slog.String("reason", "invalid_otp_code"))
		writeJSONError(w, "Ungültige Anmeldedaten", http.StatusUnauthorized)
		return
	}

	h.issueTokensAndRespond(ctx, w, user, nil)
	logging.LogAuditEvent(ctx, "AUTH_LOGIN_ADMIN_OTP", logging.AuditSuccess)
}

// issueTokensAndRespond
func (h *AdminHandlers) issueTokensAndRespond(ctx context.Context, w http.ResponseWriter, user *models.User, roles []string) {
	if roles == nil {
		roles = []string{}
	}

	accessToken, err := auth.GenerateToken(user, roles, h.PrivateKey, h.AccessTokenTTL)
	if err != nil {
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
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
	}
	writeJSONResponse(w, resp, http.StatusOK)
}

// GetGlobalProfileHandler
func (h *AdminHandlers) GetGlobalProfileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		slog.ErrorContext(ctx, "UserID nicht gefunden (Admin Profile)")
		writeJSONError(w, "Benutzeridentifikation im Kontext fehlt", http.StatusInternalServerError)
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		slog.ErrorContext(ctx, "Ungültige UserID im Kontext (Admin Profile)", "error", err)
		writeJSONError(w, "Ungültige Benutzeridentifikation", http.StatusBadRequest)
		return
	}
	user, err := h.UserRepo.GetUserByID(ctx, userID)
	if err != nil {
		handleGetUserError(ctx, w, err, userIDStr)
		return
	}

	response := ProfileResponse{
		ID:         user.ID,
		Email:      user.Email,
		Roles:      []string{"admin"},
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
		OTPEnabled: user.GlobalOTPEnabled,
	}

	writeJSONResponse(w, response, http.StatusOK)
}