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
	UsageDropOutcomeUnknown = "outcome_unknown"
	UsageDropShutdown       = "shutdown"
	UsageDropStaleSession   = "stale_session"
	UsageDropNoIMSI         = "no_imsi"
)

var (
	signalingMessages    *prometheus.CounterVec
	registrationAttempts *prometheus.CounterVec
	usageBytesDropped    *prometheus.CounterVec
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

	usageBytesDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_upf_usage_dropped_bytes_total",
		Help: "Total number of accounted usage bytes never recorded in the subscriber-usage store, by reason.",
	}, []string{"reason"})

	prometheus.MustRegister(signalingMessages, registrationAttempts, usageBytesDropped)

	for _, reason := range []string{UsageDropOutcomeUnknown, UsageDropShutdown, UsageDropStaleSession, UsageDropNoIMSI} {
		usageBytesDropped.WithLabelValues(reason)
	}
}

// RegisterRadioGauges registers the connected-radio and registered-subscriber
// gauges, each broken down by RAT. The callbacks tally the live counts from the
// AMF (5G) and MME (4G); a nil callback contributes zero, so a single-RAT
// deployment can omit the absent side.
func RegisterRadioEventDrops(dropped func() uint64) {
	prometheus.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_radio_events_dropped_total",
		Help: "Radio events not written to the database because the write buffer was full.",
	}, func() float64 { return float64(dropped()) }))
}

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

func DroppedUsage(reason string, bytes uint64) {
	if usageBytesDropped == nil {
		return
	}

	usageBytesDropped.WithLabelValues(reason).Add(float64(bytes))
}
