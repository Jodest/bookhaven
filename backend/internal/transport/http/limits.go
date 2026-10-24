package http

import (
	"net/http"
	"time"

	adminhttp "github.com/bookhaven/backend/internal/modules/admin/transport/http"
)

// DefaultBodyLimit is the request body cap applied to every route (1 MiB).
const DefaultBodyLimit int64 = 1 << 20

// BodyLimitOverrides lists the routes whose body limit is larger than
// DefaultBodyLimit, keyed "METHOD routePattern". Any new upload route must add
// its own entry here: a route-level limit cannot raise the global one.
func BodyLimitOverrides() map[string]int64 {
	return map[string]int64{
		"POST /api/v1/admin/books/import": adminhttp.ImportMaxRequestBodyBytes,
	}
}

// ServerTimeouts are the http.Server timeouts of the api (0 = no limit).
type ServerTimeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

// ApplyServerTimeouts sets the four timeout fields on s.
func ApplyServerTimeouts(s *http.Server, t ServerTimeouts) {
	s.ReadHeaderTimeout = t.ReadHeader
	s.ReadTimeout = t.Read
	s.WriteTimeout = t.Write
	s.IdleTimeout = t.Idle
}
