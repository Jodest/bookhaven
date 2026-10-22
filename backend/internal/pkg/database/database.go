// Package database opens the PostgreSQL connection and bounds its pool
// (docs/features/db-pool-limits.md).
package database

import (
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PoolConfig holds the database/sql pool limits of one process.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// PoolConfigFrom builds the pool limits from the loaded config.
func PoolConfigFrom(cfg *config.Config) PoolConfig {
	return PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
	}
}

// ApplyPool sets the pool limits on the *sql.DB underneath db.
func ApplyPool(db *gorm.DB, p PoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(p.MaxOpenConns)
	sqlDB.SetMaxIdleConns(p.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(p.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(p.ConnMaxIdleTime)
	return nil
}

// Open connects to PostgreSQL and applies the pool limits.
func Open(dsn string, p PoolConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := ApplyPool(db, p); err != nil {
		return nil, err
	}
	return db, nil
}
