package server

import (
	"athena/internal/database"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func NewServer(port string, handler http.Handler) *http.Server {
	addr := fmt.Sprintf(":%s", port)
	
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func StartAndShutdown(srv *http.Server, db *database.DB) {
	// Starten
	go func() {
		slog.Info("Auth Service startet...", slog.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Fehler beim Starten des Servers", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Warten auf Signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Fahre Server herunter (Graceful Shutdown)...")

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Fehler beim Graceful Shutdown des Servers", slog.Any("error", err))
	}

	if err := db.Close(); err != nil {
		slog.Error("Fehler beim Schließen der Datenbankverbindung", slog.Any("error", err))
	}

	slog.Info("Server erfolgreich heruntergefahren.")
}