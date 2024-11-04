package server

import (
	"net/http"
)

type middleware func(http.Handler) http.Handler

// The loggingMiddlewareContext type helps the logging middleware receive and pass along information through the middleware chain.
type loggingMiddlewareContext struct {
	responseStatusCode int
}

// The statusRecorder struct wraps the http.ResponseWriter struct, and extracts the status
// code of the response writer for the middleware to read
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter returns a new ResponseWriterCloner struct
// it returns http.StatusOK by default because the http.ResponseWriter defaults to that header
// if the WriteHeader() function is never called.
func newResponseWriter(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{w, http.StatusOK}
}

// WriteHeader overrides the ResponseWriter method to duplicate the status code into the wrapper struct
func (rwc *statusRecorder) WriteHeader(code int) {
	rwc.statusCode = code
	rwc.ResponseWriter.WriteHeader(code)
}

// createMiddlewareStack chains the given middleware functions to wrap the api.
// Each middleware functions calls next.ServeHTTP in order to resume the chain of execution.
// The order the middleware functions are given to createMiddlewareStack matters.
// Any code before next.ServeHTTP is called is executed in the given middleware's order.
// Any code after next.ServeHTTP is called is executed in the given middleware's reverse order.
func createMiddlewareStack(middleware ...middleware) middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			mw := middleware[i]
			next = mw(next)
		}
		return next
	}
}
