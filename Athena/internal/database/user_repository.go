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

// CreateUser fügt einen neuen Benutzer in die Datenbank ein.
func (r *sqlxRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (id, email, password_hash, is_admin, created_at, updated_at)
               VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		user.ID.String(),
		user.Email,
		user.PasswordHash,
		user.IsAdmin,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Einfügen des Benutzers", slog.Any("error", err), slog.String("email", user.Email))
		return err
	}
	slog.DebugContext(ctx, "Benutzer erfolgreich in DB erstellt", slog.String("user_id", user.ID.String()))
	return nil
}

// GetUserByEmail ruft einen Benutzer anhand seiner E-Mail-Adresse ab.
func (r *sqlxRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var dbUser struct {
		models.User
		IDString string `db:"id"`
	}
	query := `SELECT * FROM users WHERE email = ? LIMIT 1`
	err := r.db.GetContext(ctx, &dbUser, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Benutzer nicht gefunden", slog.String("email", email))
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des Benutzers nach E-Mail", slog.Any("error", err), slog.String("email", email))
		return nil, err
	}
	user := dbUser.User
	parsedID, parseErr := uuid.Parse(dbUser.IDString)
	if parseErr != nil {
		slog.ErrorContext(ctx, "Fehler beim Parsen der User-ID aus der DB", slog.Any("error", parseErr), slog.String("id_string", dbUser.IDString))
		return nil, parseErr
	}
	user.ID = parsedID
	slog.DebugContext(ctx, "Benutzer erfolgreich nach E-Mail gefunden", slog.String("user_id", user.ID.String()))
	return &user, nil
}

// GetUserByID ruft einen Benutzer anhand seiner ID ab.
func (r *sqlxRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var dbUser struct {
		models.User
		IDString string `db:"id"`
	}
	query := `SELECT * FROM users WHERE id = ? LIMIT 1`
	err := r.db.GetContext(ctx, &dbUser, query, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Benutzer nicht gefunden", slog.String("id", id.String()))
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des Benutzers nach ID", slog.Any("error", err), slog.String("id", id.String()))
		return nil, err
	}
	user := dbUser.User
	parsedID, parseErr := uuid.Parse(dbUser.IDString)
	if parseErr != nil {
		slog.ErrorContext(ctx, "Fehler beim Parsen der User-ID aus der DB", slog.Any("error", parseErr), slog.String("id_string", dbUser.IDString))
		return nil, parseErr
	}
	user.ID = parsedID
	slog.DebugContext(ctx, "Benutzer erfolgreich nach ID gefunden", slog.String("user_id", user.ID.String()))
	return &user, nil
}

// UpdateUser aktualisiert globale Benutzerdaten (z.B. E-Mail).
func (r *sqlxRepository) UpdateUser(ctx context.Context, user *models.User) error {
	query := `UPDATE users SET email = ?, updated_at = ? WHERE id = ?`

	now := time.Now().UTC()
	user.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.UpdatedAt,
		user.ID.String(),
	)

	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktualisieren des Benutzers", slog.Any("error", err), slog.String("user_id", user.ID.String()))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, nicht existierenden Benutzer zu aktualisieren", slog.String("user_id", user.ID.String()))
		return ErrUserNotFound
	}

	slog.DebugContext(ctx, "Benutzer erfolgreich aktualisiert", slog.String("user_id", user.ID.String()))
	return nil
}

func (r *sqlxRepository) GetUserCount(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM users`

	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Zählen der Benutzer", slog.Any("error", err))
		return 0, err
	}
	return count, nil
}

func (r *sqlxRepository) SetGlobalOTPSecretAndURL(ctx context.Context, userID uuid.UUID, secret, authURL string) error {
	query := `UPDATE users SET global_otp_secret = ?, global_otp_auth_url = ?, updated_at = ?
	           WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, secret, authURL, time.Now().UTC(), userID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Setzen des globalen OTP-Secrets/URL", "error", err, "user_id", userID.String())
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	slog.DebugContext(ctx, "Globales OTP-Secret und URL erfolgreich gesetzt", "user_id", userID.String())
	return nil
}

func (r *sqlxRepository) EnableGlobalOTP(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET global_otp_enabled = TRUE, updated_at = ?
	           WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), userID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Aktivieren von globalem OTP", "error", err, "user_id", userID.String())
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	slog.InfoContext(ctx, "Globales OTP erfolgreich aktiviert", "user_id", userID.String())
	return nil
}

func (r *sqlxRepository) DisableGlobalOTP(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET global_otp_enabled = FALSE, global_otp_secret = NULL, global_otp_auth_url = NULL, updated_at = ?
	           WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, time.Now().UTC(), userID.String())
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Deaktivieren von globalem OTP", "error", err, "user_id", userID.String())
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	slog.InfoContext(ctx, "Globales OTP erfolgreich deaktiviert", "user_id", userID.String())
	return nil
}

func (r *sqlxRepository) GetGlobalOTPSecretAndStatus(ctx context.Context, userID uuid.UUID) (secret sql.NullString, isEnabled bool, err error) {
	query := `SELECT global_otp_secret, global_otp_enabled FROM users
	           WHERE id = ? LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, userID.String())
	err = row.Scan(&secret, &isEnabled)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Benutzer für globales OTP-Statusabfrage nicht gefunden", "user_id", userID.String())
			return secret, false, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des globalen OTP-Status/Secrets", "error", err, "user_id", userID.String())
		return secret, false, err
	}

	slog.DebugContext(ctx, "Globales OTP-Status/Secret erfolgreich abgerufen", "user_id", userID.String(), "enabled", isEnabled)
	return secret, isEnabled, nil
}