package server

import (
	"net/http"
)

func NewEllaRouter(config *HandlerConfig) http.Handler {
	apiV1Router := http.NewServeMux()

	apiV1Router.HandleFunc("GET /subscribers", ListSubscribers(config))
	apiV1Router.HandleFunc("GET /subscribers/{id}", GetSubscriber(config))
	apiV1Router.HandleFunc("DELETE /subscribers/{id}", DeleteSubscriber(config))
	apiV1Router.HandleFunc("POST /subscribers", CreateSubscriber(config))

	frontendHandler := newFrontendFileServer()

	router := http.NewServeMux()
	ctx := loggingMiddlewareContext{}
	apiMiddlewareStack := createMiddlewareStack(
		loggingMiddleware(&ctx),
	)
	metricsMiddlewareStack := createMiddlewareStack()
	router.Handle("/api/v1/", http.StripPrefix("/api/v1", apiMiddlewareStack(apiV1Router)))
	router.Handle("/", metricsMiddlewareStack(frontendHandler))

	return router
}
