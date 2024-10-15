package server

import (
	"net/http"
)

func NewEllaRouter(config *HandlerConfig) http.Handler {
	apiV1Router := http.NewServeMux()

	// Inventory (Radios)
	apiV1Router.HandleFunc("GET /inventory/radios", ListRadios(config))
	apiV1Router.HandleFunc("GET /inventory/radios/{id}", GetRadio(config))
	apiV1Router.HandleFunc("DELETE /inventory/radios/{id}", DeleteRadio(config))
	apiV1Router.HandleFunc("POST /inventory/radios", CreateRadio(config))

	// Subscribers
	apiV1Router.HandleFunc("GET /subscribers", ListSubscribers(config))
	apiV1Router.HandleFunc("GET /subscribers/{id}", GetSubscriber(config))
	apiV1Router.HandleFunc("DELETE /subscribers/{id}", DeleteSubscriber(config))
	apiV1Router.HandleFunc("POST /subscribers", CreateSubscriber(config))

	// Device Groups
	apiV1Router.HandleFunc("GET /device-groups", ListDeviceGroups(config))
	apiV1Router.HandleFunc("GET /device-groups/{id}", GetDeviceGroup(config))
	apiV1Router.HandleFunc("DELETE /device-groups/{id}", DeleteDeviceGroup(config))
	apiV1Router.HandleFunc("POST /device-groups", CreateDeviceGroup(config))

	// Network Slices
	apiV1Router.HandleFunc("GET /network-slices", ListNetworkSlices(config))
	apiV1Router.HandleFunc("GET /network-slices/{id}", GetNetworkSlice(config))
	apiV1Router.HandleFunc("DELETE /network-slices/{id}", DeleteNetworkSlice(config))
	apiV1Router.HandleFunc("POST /network-slices", CreateNetworkSlice(config))

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
