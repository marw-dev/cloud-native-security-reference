package handlers

import (
	"athena/internal/database"
	"log/slog"
	"net/http"
)

type InternalHandlers struct {
	RouteRepo   database.RouteRepository
	ProjectRepo database.ProjectRepository
}

func NewInternalHandlers(routeRepo database.RouteRepository, projectRepo database.ProjectRepository) *InternalHandlers {
	return &InternalHandlers{
		RouteRepo:   routeRepo,
		ProjectRepo: projectRepo,
	}
}

func (h *InternalHandlers) GetAllRoutesConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	routes, err := h.RouteRepo.GetAllProjectRoutes(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "[INTERNAL] Fehler beim Abrufen aller Routen für Aegis", slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, routes, http.StatusOK)
}

// GetContextMapHandler
func (h *InternalHandlers) GetContextMapHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	contextMap, err := h.ProjectRepo.GetProjectContextMap(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "[INTERNAL] Fehler beim Abrufen der Context Map", slog.Any("error", err))
		writeJSONError(w, "Interner Serverfehler", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, contextMap, http.StatusOK)
}