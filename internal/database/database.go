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

	"github.com/dotinschool/dotinschool-challenge/internal/config"
	"github.com/dotinschool/dotinschool-challenge/internal/model"
)

// Open connects to arena.db (creating the data directory if needed), applies
// pragmatic pragmas, runs AutoMigrate, and returns the handle.
func Open(cfg *config.Config, log *logrus.Logger) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, err
	}
	db, err := openFile(cfg.DBPath(), log)
	if err != nil {
		return nil, err
	}
	// The file holds every entrant's phone number and the panel's API keys —
	// owner-only, like the .env and sidecar files next to it. Best effort: a
	// bind mount owned by someone else may refuse, which must not stop boot.
	_ = os.Chmod(cfg.DBPath(), 0o600)
	return db, nil
}

// buildDSN is the connection string for a SQLite file at path, with the exact
// pragmas the live database uses — so a file that opens here will open the same
// way on the next boot.
//
// mmap_size(0) + a rollback journal instead of WAL: the pure-Go SQLite driver
// memory-maps the database (and WAL mode additionally mmaps a shared -shm file)
// - which fails with ENOMEM, surfacing as SQLITE_CANTOPEN ("unable to open
// database file: out of memory (14)"), inside some Docker containers even on a
// host with plenty of free RAM. Plain pread/pwrite has none of that dependency;
// TRUNCATE keeps a fast-ish journal without needing the -shm mapping. For this
// single-instance workload the concurrency tradeoff is irrelevant.
func buildDSN(path string) string {
	return path + "?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(TRUNCATE)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=mmap_size(0)"
}

// openFile opens and migrates the SQLite database at an explicit path.
func openFile(path string, log *logrus.Logger) (*gorm.DB, error) {
	gormLog := glogger.New(log, glogger.Config{
		SlowThreshold:             300 * time.Millisecond,
		LogLevel:                  glogger.Warn,
		IgnoreRecordNotFoundError: true, // "key not found" lookups are routine here
		Colorful:                  false,
	})

	db, err := gorm.Open(sqlite.Open(buildDSN(path)), &gorm.Config{
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

// sidecarSuffixes are the auxiliary files SQLite keeps next to a database file.
var sidecarSuffixes = []string{"-journal", "-wal", "-shm"}

// CleanSidecars deletes any stale -journal / -wal / -shm files next to path.
// Call it immediately before swapping a database file into place: a hot journal
// left over from the *previous* database would otherwise be replayed by SQLite
// against the new file on the next open — "database disk image is malformed",
// then a crash loop. (The app runs in journal_mode(TRUNCATE), so the file that
// actually matters here is "-journal".)
func CleanSidecars(path string) {
	for _, s := range sidecarSuffixes {
		_ = os.Remove(path + s)
	}
}

// Validate proves that the SQLite file at path can become THIS build's working
// database: it opens the file with the live pragmas, runs AutoMigrate (which
// also brings a slightly older backup up to the current schema, in place), and
// smoke-queries the core tables. An error means the file is corrupt or its
// schema cannot be reconciled — the caller must refuse the restore rather than
// swap it in and crash-loop on the next boot. Leaves no sidecar files behind.
func Validate(path string, log *logrus.Logger) error {
	db, err := openFile(path, log)
	if err != nil {
		CleanSidecars(path)
		return err
	}
	var n int64
	err = db.Model(&model.Participant{}).Count(&n).Error
	if err == nil {
		err = db.Model(&model.Setting{}).Count(&n).Error
	}
	if err == nil {
		err = db.Model(&model.ClientError{}).Count(&n).Error
	}
	Close(db)
	CleanSidecars(path)
	return err
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
