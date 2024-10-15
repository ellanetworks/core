package server

import (
	"net/http"
)

func NewEllaRouter(config *HandlerConfig) http.Handler {
	apiV1Router := http.NewServeMux()

	// Inventory (GNBs)
	apiV1Router.HandleFunc("GET /inventory/gnbs", ListGnbs(config))
	apiV1Router.HandleFunc("GET /inventory/gnbs/{id}", GetGnb(config))
	apiV1Router.HandleFunc("DELETE /inventory/gnbs/{id}", DeleteGnb(config))
	apiV1Router.HandleFunc("POST /inventory/gnbs", CreateGnb(config))

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

	// Device Group subscribers
	apiV1Router.HandleFunc("GET /device-groups/{device_group_id}/subscribers", ListDeviceGroupSubscribers(config))
	apiV1Router.HandleFunc("POST /device-groups/{device_group_id}/subscribers", CreateDeviceGroupSubscriber(config))
	apiV1Router.HandleFunc("DELETE /device-groups/{device_group_id}/subscribers/{subscriber_id}", DeleteDeviceGroupSubscriber(config))

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
