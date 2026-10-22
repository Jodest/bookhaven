package database

import (
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/config"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestApplyPoolSetsOpenLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	p := PoolConfig{MaxOpenConns: 7, MaxIdleConns: 3, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Second}
	if err := ApplyPool(db, p); err != nil {
		t.Fatalf("ApplyPool: %v", err)
	}
	sqlDB, _ := db.DB()
	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", got)
	}
}

func TestPoolConfigFrom(t *testing.T) {
	cfg := &config.Config{DBMaxOpenConns: 9, DBMaxIdleConns: 2, DBConnMaxLifetime: time.Hour, DBConnMaxIdleTime: time.Minute}
	got := PoolConfigFrom(cfg)
	want := PoolConfig{MaxOpenConns: 9, MaxIdleConns: 2, ConnMaxLifetime: time.Hour, ConnMaxIdleTime: time.Minute}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
