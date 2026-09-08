// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package metrics owns the Prometheus metrics emitted from more than one network
// function, so a single series is registered once and incremented by both the AMF
// (5G) and the MME (4G). Single-NF metrics stay in their own packages.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// RAT label values distinguishing the radio access technology a metric belongs to.
const (
	RAT4G = "4g"
	RAT5G = "5g"
)

// Result label values for attempt-style counters.
const (
	ResultAccept = "accept"
	ResultReject = "reject"
)

const (
	CollectorDatabaseStorage     = "database_storage"
	CollectorDatabaseIPTotal     = "database_ip_addresses_total"
	CollectorDatabaseIPAllocated = "database_ip_addresses_allocated"
	CollectorUPFThroughput       = "upf_throughput"
	CollectorUPFDlBuffer         = "upf_dl_buffer"
	CollectorUPFRingbuf          = "upf_ringbuf"
	CollectorUPFDatapath         = "upf_datapath"
	CollectorUPFRoute            = "upf_route"
	CollectorUPFProfiling        = "upf_profiling"
)

var allCollectors = []string{
	CollectorDatabaseStorage,
	CollectorDatabaseIPTotal,
	CollectorDatabaseIPAllocated,
	CollectorUPFThroughput,
	CollectorUPFDlBuffer,
	CollectorUPFRingbuf,
	CollectorUPFDatapath,
	CollectorUPFRoute,
	CollectorUPFProfiling,
}

var collectionErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_metrics_collection_errors_total",
	Help: "Total failed metric collections by collector. A failed collection omits its series rather than publishing a zero, so this counter is the signal that a value is missing because it could not be read.",
}, []string{"collector"})

func CollectionError(collector string) {
	collectionErrors.WithLabelValues(collector).Inc()
}

var (
	signalingMessages    *prometheus.CounterVec
	registrationAttempts *prometheus.CounterVec
)

// RegisterMetrics registers the cross-NF metrics. Called once at startup.
func RegisterMetrics() {
	signalingMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_signaling_messages_total",
		Help: "Total radio signaling messages by RAT (NGAP for 5G, S1AP for 4G), direction, and type.",
	}, []string{"rat", "direction", "type"})

	registrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_registration_attempts_total",
		Help: "Total UE registration (5G) and attach/tracking-area-update (4G) attempts by RAT, type, and result.",
	}, []string{"rat", "type", "result"})

	for _, collector := range allCollectors {
		collectionErrors.WithLabelValues(collector)
	}

	prometheus.MustRegister(signalingMessages, registrationAttempts, collectionErrors)
}

// RegisterRadioGauges registers the connected-radio and registered-subscriber
// gauges, each broken down by RAT. The callbacks tally the live counts from the
// AMF (5G) and MME (4G); a nil callback contributes zero, so a single-RAT
// deployment can omit the absent side.
func RegisterRadioGauges(radios5G, subscribers5G, radios4G, subscribers4G func() int) {
	radiosDesc := prometheus.NewDesc(
		"app_connected_radios",
		"Number of radios currently connected to Ella Core, by RAT (5G gNBs, 4G eNBs).",
		[]string{"rat"},
		nil,
	)

	subscribersDesc := prometheus.NewDesc(
		"app_registered_subscribers",
		"Number of subscribers currently registered in Ella Core, by RAT (5GS, EPS).",
		[]string{"rat"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(countOrZero(radios5G)), RAT5G)

		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(countOrZero(radios4G)), RAT4G)

		ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(countOrZero(subscribers5G)), RAT5G)

		ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(countOrZero(subscribers4G)), RAT4G)
	}))
}

func countOrZero(f func() int) int {
	if f == nil {
		return 0
	}

	return f()
}

// SignalingMessage records one NGAP/S1AP message. rat is RAT4G or RAT5G; direction
// is "inbound" or "outbound"; msgType is the procedure/message name.
func SignalingMessage(rat, direction, msgType string) {
	if signalingMessages == nil {
		return
	}

	signalingMessages.WithLabelValues(rat, direction, msgType).Inc()
}

// RegistrationAttempt records one registration/attach/TAU outcome. rat is RAT4G or
// RAT5G; regType is the 3GPP procedure name; result is ResultAccept or ResultReject.
func RegistrationAttempt(rat, regType, result string) {
	if registrationAttempts == nil {
		return
	}

	registrationAttempts.WithLabelValues(rat, regType, result).Inc()
}
