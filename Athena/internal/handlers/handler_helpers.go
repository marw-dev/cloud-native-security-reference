package handlers

import (
	"athena/internal/database"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
)

// Die 'validate'-Instanz lebt jetzt hier
var validate = validator.New(validator.WithRequiredStructEnabled())

// init() registriert alle benutzerdefinierten Validatoren
func init() {
	validate.RegisterValidation("duration", durationValidator)
}

func durationValidator(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true // Leere Strings sind okay (bedeutet 'aus')
	}
	_, err := time.ParseDuration(fl.Field().String())
	return err == nil
}

// validateRequest (aus auth_handlers.go verschoben)
func validateRequest(ctx context.Context, req interface{}) map[string]string {
	err := validate.StructCtx(ctx, req)
	if err == nil {
		return nil
	}

	validationErrors := err.(validator.ValidationErrors)
	errorMessages := make(map[string]string)

	for _, fieldErr := range validationErrors {
		fieldName := fieldErr.Field()
		switch fieldErr.Tag() {
		case "required":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' ist erforderlich.", fieldName)
		case "email":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' muss eine gültige E-Mail-Adresse sein.", fieldName)
		case "min":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' muss mindestens %s Zeichen lang sein.", fieldName, fieldErr.Param())
		case "numeric":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' muss numerisch sein.", fieldName)
		case "len":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' muss genau %s Zeichen lang sein.", fieldName, fieldErr.Param())
		case "duration":
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' muss eine gültige Dauer sein (z.B. '30s', '1m').", fieldName)
		default:
			errorMessages[fieldName] = fmt.Sprintf("Feld '%s' ist ungültig (%s).", fieldName, fieldErr.Tag())
		}
	}
	return errorMessages
}

// writeJSONError (aus auth_handlers.go verschoben)
func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// writeJSONResponse (aus auth_handlers.go verschoben)
func writeJSONResponse(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Fehler beim Senden der JSON-Antwort", slog.Any("error", err))
	}
}

// handleGetUserError (aus auth_handlers.go verschoben)
func handleGetUserError(ctx context.Context, w http.ResponseWriter, err error, identifier string) {
	if errors.Is(err, database.ErrUserNotFound) || errors.Is(err, database.ErrUserNotInProject) {
		slog.WarnContext(ctx, "Benutzer nicht gefunden oder nicht im Projekt", slog.String("identifier", identifier), slog.Any("error", err))
		writeJSONError(w, "Ungültige Anmeldedaten", http.StatusUnauthorized)
	} else {
		slog.ErrorContext(ctx, "Fehler beim Abrufen des Benutzers/Projekts", slog.Any("error", err), slog.String("identifier", identifier))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
	}
}

func HealthCheckHandler(pinger database.DBPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := pinger.PingContext(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "Health Check fehlgeschlagen: DB nicht erreichbar", slog.Any("error", err))
			http.Error(w, "NOK", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}