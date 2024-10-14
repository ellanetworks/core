package server

import (
	"net/http"
)

// The Logging middleware captures any http request coming through and the response status code, and logs it.
func loggingMiddleware(ctx *loggingMiddlewareContext) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clonedWriter := newResponseWriter(w)
			next.ServeHTTP(clonedWriter, r)
			ctx.responseStatusCode = clonedWriter.statusCode
		})
	}
}
