package http

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	healthStatusOK          = "ok"
	healthStatusUnavailable = "unavailable"
)

// HealthChecker is a single dependency probe used by the readiness endpoint.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

type gormHealthChecker struct{ db *gorm.DB }

// NewGormHealthChecker pings PostgreSQL through the existing GORM pool.
func NewGormHealthChecker(db *gorm.DB) HealthChecker { return gormHealthChecker{db: db} }

func (gormHealthChecker) Name() string { return "database" }

func (g gormHealthChecker) Check(ctx context.Context) error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

type redisHealthChecker struct{ client *redis.Client }

// NewRedisHealthChecker sends PING over the existing go-redis client.
func NewRedisHealthChecker(client *redis.Client) HealthChecker {
	return redisHealthChecker{client: client}
}

func (redisHealthChecker) Name() string { return "redis" }

func (r redisHealthChecker) Check(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// RegisterHealthRoutes registers the public liveness and readiness endpoints.
// Each readiness check runs concurrently under its own timeout.
func RegisterHealthRoutes(e *echo.Echo, logger *zap.Logger, timeout time.Duration, checks ...HealthChecker) {
	e.GET("/health", healthLiveness)
	e.GET("/health/ready", func(c echo.Context) error {
		return healthReadiness(c, logger, timeout, checks)
	})
}

// healthLiveness is the static liveness probe: no dependency is checked.
func healthLiveness(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, healthResponse{Status: healthStatusOK})
}

// healthReadiness pings every dependency; the body carries only ok/unavailable,
// failure reasons go to the log. Not in swagger: BasePath is /api/v1.
func healthReadiness(c echo.Context, logger *zap.Logger, timeout time.Duration, checks []HealthChecker) error {
	results := make([]string, len(checks))
	var wg sync.WaitGroup
	for i, chk := range checks {
		wg.Add(1)
		go func(i int, chk HealthChecker) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()

			// Run the probe separately so a driver that ignores ctx cannot hang the request.
			done := make(chan error, 1)
			go func() { done <- chk.Check(ctx) }()

			var err error
			select {
			case err = <-done:
			case <-ctx.Done():
				err = ctx.Err()
			}
			if err != nil {
				logger.Warn("readiness check failed", zap.String("check", chk.Name()), zap.Error(err))
				results[i] = healthStatusUnavailable
				return
			}
			results[i] = healthStatusOK
		}(i, chk)
	}
	wg.Wait()

	resp := healthResponse{Status: healthStatusOK, Checks: make(map[string]string, len(checks))}
	code := http.StatusOK
	for i, chk := range checks {
		resp.Checks[chk.Name()] = results[i]
		if results[i] != healthStatusOK {
			resp.Status = healthStatusUnavailable
			code = http.StatusServiceUnavailable
		}
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(code, resp)
}
