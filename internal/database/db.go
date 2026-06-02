// Package database provides database initialization and connection management.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/example/app-tpl/internal/config"
	"github.com/example/app-tpl/internal/constant"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	_ "github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
	"go.uber.org/zap"
)

// DB wraps the Bun database instance.
type DB struct {
	*bun.DB
}

// NewDB creates a new database connection based on the configuration.
func NewDB(cfg *config.Config, log *zap.Logger) (*DB, error) {
	var sqldb *sql.DB
	var db *bun.DB

	switch cfg.Database.Driver {
	case constant.DriverMySQL:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
		)
		var err error
		sqldb, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open mysql connection: %w", err)
		}
		db = bun.NewDB(sqldb, mysqldialect.New())

	case constant.DriverPostgres, constant.DriverPostgreSQL:
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
			cfg.Database.SSLMode,
		)
		sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		db = bun.NewDB(sqldb, pgdialect.New())

	case constant.DriverSQLite, constant.DriverSQLite3:
		dbPath := cfg.Database.Database
		var dsn string
		// Enable WAL mode, foreign keys and busy timeout for SQLite
		if dbPath == "" || dbPath == ":memory:" {
			dsn = "file::memory:?cache=shared"
		} else {
			// Auto-create parent directory if it does not exist
			if dir := filepath.Dir(dbPath); dir != "." {
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
				}
			}
			// Use file: URI so query parameters work with the CGO-free sqliteshim driver
			dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
		}
		var err error
		sqldb, err = sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
		}
		db = bun.NewDB(sqldb, sqlitedialect.New())

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	// Add query logger in debug mode
	if cfg.Log.Level == constant.LogLevelDebug {
		db.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithVerbose(true),
			bundebug.FromEnv("BUNDEBUG"),
		))
	}

	// Configure connection pool
	maxOpenConns := cfg.Database.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 25
	}
	db.SetMaxOpenConns(maxOpenConns)

	maxIdleConns := cfg.Database.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10
	}
	db.SetMaxIdleConns(maxIdleConns)

	connMaxLife := cfg.Database.ConnMaxLife
	if connMaxLife <= 0 {
		connMaxLife = 300 // 5 minutes in seconds
	}
	db.SetConnMaxLifetime(time.Duration(connMaxLife) * time.Second)

	// Set default connection max idle time
	connMaxIdleTime := cfg.Database.ConnMaxIdle
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 60 // 1 minute in seconds
	}
	db.SetConnMaxIdleTime(time.Duration(connMaxIdleTime) * time.Second)

	// Verify connection is alive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqldb.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connected",
		zap.String("driver", cfg.Database.Driver),
		zap.String("database", cfg.Database.Database),
		zap.Int("max_open_conns", maxOpenConns),
	)

	return &DB{DB: db}, nil
}

// HealthCheck checks the database connection health.
func (db *DB) HealthCheck(ctx context.Context) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}

	return db.PingContext(ctx)
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db == nil {
		return nil
	}
	return db.DB.Close()
}
