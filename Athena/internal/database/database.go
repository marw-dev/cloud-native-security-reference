package database

import (
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DB struct {
	*sqlx.DB
}

func ConnectDB(databaseURL string) (*DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL darf nicht leer sein")
	}

	db, err := sqlx.Connect("mysql", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("fehler beim Verbinden zur Datenbank: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("fehler beim Pingen der Datenbank: %w", err)
	}

	slog.Info("Erfolgreich mit der Datenbank verbunden")
	return &DB{db}, nil
}

func (db *DB) Close() error {
	slog.Info("Schließe Datenbankverbindung...")
	return db.DB.Close()
}