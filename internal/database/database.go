// Package database opens the single SQLite file, runs migrations, and seeds a
// fresh install with sane defaults and a handful of demo participants.
package database

import (
	"errors"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"

	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/model"
)

// Open connects to arena.db (creating the data directory if needed), applies
// pragmatic pragmas, runs AutoMigrate, and returns the handle.
func Open(cfg *config.Config, log *logrus.Logger) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	gormLog := glogger.New(log, glogger.Config{
		SlowThreshold:             300 * time.Millisecond,
		LogLevel:                  glogger.Warn,
		IgnoreRecordNotFoundError: true, // "key not found" lookups are routine here
		Colorful:                  false,
	})

	// mmap_size(0) + a rollback journal instead of WAL: the pure-Go SQLite
	// driver memory-maps the database (and WAL mode additionally mmaps a shared
	// -shm file) - which fails with ENOMEM, surfacing as SQLITE_CANTOPEN
	// ("unable to open database file: out of memory (14)"), inside some
	// Docker containers even on a host with plenty of free RAM (the same
	// binary run directly on that host opens the file fine). Plain
	// pread/pwrite has none of that dependency; TRUNCATE keeps a fast-ish
	// journal without needing the -shm mapping. For this single-instance
	// workload the concurrency tradeoff is irrelevant.
	dsn := cfg.DBPath() + "?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(TRUNCATE)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=mmap_size(0)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 gormLog,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// One connection: SQLite is single-writer anyway, and with a rollback
	// journal (not WAL) concurrent connections just contend on SQLITE_BUSY.
	if sqlDB, derr := db.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	if err := Migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate creates/updates every table. GORM's AutoMigrate adds missing columns
// in place, which covers the "lightweight migration for older databases" the
// original server did by hand.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Participant{},
		&model.Setting{},
		&model.ClientError{},
	)
}

// Close releases the underlying connection pool.
func Close(db *gorm.DB) {
	if db == nil {
		return
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// IsNotFound is a small helper for handlers.
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
