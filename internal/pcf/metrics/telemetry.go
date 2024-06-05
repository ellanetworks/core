/*
 *  Metrics package is used to expose the metrics of the PCF service.
 */

package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yeastengine/canard/internal/pcf/logger"
)

// InitMetrics initializes PCF metrics
func InitMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.InitLog.Errorf("Could not open metrics port: %v", err)
	}
}
