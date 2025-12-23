package database

import (
	"database/sql"
	"errors"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(db *sql.DB, dbName string) error {
	slog.Info("Starte Datenbank-Migrationen...")

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file:///migrations",
		dbName,
		driver,
	)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("Datenbank ist bereits auf dem neuesten Stand.")
			return nil
		}
		return err
	}

	slog.Info("Datenbank-Migrationen erfolgreich abgeschlossen.")
	return nil
}