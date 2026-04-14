package server

import (
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
)

// appliedIndexWriter defers WriteHeader until after the handler's status is
// known, so the middleware can stamp X-Ella-Applied-Index just in time.
type appliedIndexWriter struct {
	http.ResponseWriter
	dbInstance  *db.Database
	wroteHeader bool
}

func (w *appliedIndexWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.wroteHeader = true

		// Stamp the leader's currently applied index so proxying followers
		// can block their local reads until they've caught up.
		w.Header().Set(headerAppliedIndex, strconv.FormatUint(w.dbInstance.RaftAppliedIndex(), 10))
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *appliedIndexWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(b)
}

// AppliedIndexMiddleware emits the leader's last-applied Raft index on write
// responses when clustering is enabled, enabling the leader-proxy RYW wait.
func AppliedIndexMiddleware(dbInstance *db.Database, next http.Handler) http.Handler {
	if dbInstance == nil || !dbInstance.ClusterEnabled() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWriteMethod(r.Method) || !dbInstance.IsLeader() {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(&appliedIndexWriter{ResponseWriter: w, dbInstance: dbInstance}, r)
	})
}
