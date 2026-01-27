// Copyright 2026 Ella Networks

package context

import (
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics() {
	connectedRadios := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_connected_radios",
		Help: "Number of radios currently connected to Ella Core",
	}, func() float64 {
		return float64(AMFSelf().CountRadios())
	})

	registeredSubscribers := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_registered_subscribers",
		Help: "Number of subscribers currently registered in Ella Core",
	}, func() float64 {
		return float64(AMFSelf().CountRegisteredSubscribers())
	})

	prometheus.MustRegister(connectedRadios)
	prometheus.MustRegister(registeredSubscribers)
}
