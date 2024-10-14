package server

import (
	"net/http"
)

func NewEllaRouter(config *HandlerConfig) http.Handler {
	apiV1Router := http.NewServeMux()

	// Subscribers
	apiV1Router.HandleFunc("GET /subscribers", ListSubscribers(config))
	apiV1Router.HandleFunc("GET /subscribers/{id}", GetSubscriber(config))
	apiV1Router.HandleFunc("DELETE /subscribers/{id}", DeleteSubscriber(config))
	apiV1Router.HandleFunc("POST /subscribers", CreateSubscriber(config))

	// Device groups
	apiV1Router.HandleFunc("GET /device-groups", ListDeviceGroups(config))
	apiV1Router.HandleFunc("GET /device-groups/{id}", GetDeviceGroup(config))
	apiV1Router.HandleFunc("DELETE /device-groups/{id}", DeleteDeviceGroup(config))
	apiV1Router.HandleFunc("POST /device-groups", CreateDeviceGroup(config))

	// Device group subscribers
	apiV1Router.HandleFunc("GET /device-groups/{device_group_id}/subscribers", ListDeviceGroupSubscribers(config))
	apiV1Router.HandleFunc("POST /device-groups/{device_group_id}/subscribers", CreateDeviceGroupSubscriber(config))
	apiV1Router.HandleFunc("DELETE /device-groups/{device_group_id}/subscribers/{subscriber_id}", DeleteDeviceGroupSubscriber(config))

	// Frontend
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
