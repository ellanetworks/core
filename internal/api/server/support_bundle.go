package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const SupportBundleAction = "support_bundle:generate"

// SupportBundle handler streams a gzipped tar support bundle assembled from DB.
func SupportBundle(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)
		emailStr, _ := email.(string)

		ctxVar := r.Context()

		pr, pw := io.Pipe()
		errCh := make(chan error, 1)
		start := time.Now()

		go func(ctx context.Context) {
			err := dbInstance.WriteSupportBundle(ctx, pw)
			if err != nil {
				_ = pw.CloseWithError(err)
			} else {
				_ = pw.Close()
			}

			errCh <- err
		}(ctxVar)

		firstBuf := make([]byte, 1024)

		n, readErr := pr.Read(firstBuf)
		if readErr != nil && n == 0 {
			genErr := <-errCh
			writeError(ctxVar, w, http.StatusInternalServerError, "Failed to generate support bundle", genErr, logger.APILog)

			return
		}

		filename := fmt.Sprintf("ella-support-%s.tar.gz", time.Now().Format("20060102_150405"))
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Type", "application/gzip")

		cw := &countingWriter{w: w}

		if n > 0 {
			if _, err := cw.Write(firstBuf[:n]); err != nil {
				return
			}
		}

		if _, err := io.Copy(cw, pr); err != nil {
			logger.APILog.Warn("failed while streaming support bundle to client", zap.Error(err))
		}

		// wait for generator completion and log audit on success with size/duration
		genErr := <-errCh
		duration := time.Since(start)

		size := cw.n
		if genErr == nil {
			details := fmt.Sprintf("Generated support bundle size=%d duration=%s", size, duration)
			logger.LogAuditEvent(ctxVar, SupportBundleAction, emailStr, getClientIP(r), details)
			logger.APILog.Info("Generated support bundle", zap.Int64("size_bytes", size), zap.Duration("duration", duration), zap.String("user", emailStr))
		} else {
			logger.DBLog.Error("support bundle generation failed after streaming", zap.Error(genErr))
		}
	})
}

// countingWriter wraps an io.Writer and counts bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.n += int64(n)

	return n, err
}
