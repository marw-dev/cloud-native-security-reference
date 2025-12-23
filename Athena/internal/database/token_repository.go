package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"

	"github.com/google/uuid"
)

// HashToken (gehört logisch hierher)
func HashToken(token string) string {
	hasher := sha256.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (r *sqlxRepository) SaveRefreshToken(ctx context.Context, rt *RefreshToken) error {
	query := `INSERT INTO refresh_tokens (token_hash, user_id, expires_at, revoked, created_at)
	           VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		rt.TokenHash,
		rt.UserID.String(),
		rt.ExpiresAt,
		rt.Revoked,
		rt.CreatedAt,
	)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Speichern des Refresh Tokens", slog.Any("error", err), slog.String("user_id", rt.UserID.String()))
		return err
	}
	slog.DebugContext(ctx, "Refresh Token erfolgreich gespeichert", slog.String("user_id", rt.UserID.String()))
	return nil
}

func (r *sqlxRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rtDB struct {
		RefreshToken
		UserIDString string `db:"user_id"`
	}

	query := `SELECT token_hash, user_id, expires_at, revoked, created_at
	           FROM refresh_tokens WHERE token_hash = ? LIMIT 1`

	err := r.db.GetContext(ctx, &rtDB, query, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Refresh Token nicht gefunden", slog.String("hash", tokenHash))
			return nil, ErrRefreshTokenNotFound
		}
		slog.ErrorContext(ctx, "Fehler beim Abrufen des Refresh Tokens", slog.Any("error", err), slog.String("hash", tokenHash))
		return nil, err
	}

	rt := rtDB.RefreshToken
	parsedID, parseErr := uuid.Parse(rtDB.UserIDString)
	if parseErr != nil {
		slog.ErrorContext(ctx, "Fehler beim Parsen der User-ID aus Refresh Token DB", slog.Any("error", parseErr), slog.String("id_string", rtDB.UserIDString))
		return nil, parseErr
	}
	rt.UserID = parsedID

	slog.DebugContext(ctx, "Refresh Token erfolgreich gefunden", slog.String("hash", tokenHash))
	return &rt, nil
}

func (r *sqlxRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = ?`
	result, err := r.db.ExecContext(ctx, query, tokenHash)
	if err != nil {
		slog.ErrorContext(ctx, "Fehler beim Löschen des Refresh Tokens", slog.Any("error", err), slog.String("hash", tokenHash))
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.WarnContext(ctx, "Versuch, nicht existierenden Refresh Token zu löschen", slog.String("hash", tokenHash))
		return ErrRefreshTokenNotFound
	}

	slog.DebugContext(ctx, "Refresh Token erfolgreich gelöscht", slog.String("hash", tokenHash))
	return nil
}