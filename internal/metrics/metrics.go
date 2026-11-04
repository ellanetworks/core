// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package metrics owns the Prometheus metrics emitted from more than one network
// function, so a single series is registered once and incremented by both the AMF
// (5G) and the MME (4G). Single-NF metrics stay in their own packages.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RAT label values distinguishing the radio access technology a metric belongs to.
const (
	RAT4G = "4g"
	RAT5G = "5g"
)

const (
	RoamingFalse = "false"
	RoamingTrue  = "true"
)

// Result label values for attempt-style counters.
const (
	ResultAccept = "accept"
	ResultReject = "reject"
)

const (
	PeerN32 = "n32"
	PeerS6a = "s6a"
)

const (
	PeerSuccess = "success"
	PeerFailure = "failure"
	PeerError   = "error"
	PeerTimeout = "timeout"
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
	peerRequests         *prometheus.CounterVec
	peerDuration         *prometheus.HistogramVec
)

// RegisterMetrics registers the cross-NF metrics. Called once at startup.
func RegisterMetrics() {
	signalingMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_signaling_messages_total",
		Help: "Total radio signaling messages by RAT (NGAP for 5G, S1AP for 4G), direction, and type.",
	}, []string{"rat", "direction", "type"})

	registrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_registration_attempts_total",
		Help: "Total UE registration (5G) and attach/tracking-area-update (4G) attempts by RAT, type, result, and whether the subscriber is a roamer.",
	}, []string{"rat", "type", "result", "roaming"})

	usageBytesDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_upf_usage_dropped_bytes_total",
		Help: "Total number of accounted usage bytes never recorded in the subscriber-usage store, by reason.",
	}, []string{"reason"})

	peerRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_peer_requests_total",
		Help: "Requests sent to the home network of roaming partners, by interface (n32 for 5G through the SEPP, s6a for 4G) and result: success, failure (answered with an error), error (not answered) and timeout.",
	}, []string{"interface", "result"})

	peerDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_peer_request_duration_seconds",
		Help:    "Duration of requests sent to the home network of roaming partners, by interface.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"interface"})

	prometheus.MustRegister(signalingMessages, registrationAttempts, usageBytesDropped, peerRequests, peerDuration)

	for _, iface := range []string{PeerN32, PeerS6a} {
		peerDuration.WithLabelValues(iface)

		for _, result := range []string{PeerSuccess, PeerFailure, PeerError, PeerTimeout} {
			peerRequests.WithLabelValues(iface, result)
		}
	}

	for _, reason := range []string{UsageDropOutcomeUnknown, UsageDropShutdown, UsageDropStaleSession, UsageDropNoIMSI} {
		usageBytesDropped.WithLabelValues(reason)
	}
}

// RegisterRadioGauges registers the connected-radio and registered-subscriber
// gauges, each broken down by RAT. The callbacks tally the live counts from the
// AMF (5G) and MME (4G); a nil callback contributes zero, so a single-RAT
// deployment can omit the absent side.
func RegisterRoamingPartnerGauge(n32, s6a func() int) {
	desc := prometheus.NewDesc(
		"app_connected_roaming_partners",
		"Number of roaming partners connected to this node, by interface (n32: N32-c context established, s6a: Diameter peer open).",
		[]string{"interface"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(countOrZero(n32)), PeerN32)

		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(countOrZero(s6a)), PeerS6a)
	}))
}

func RegisterRadioEventDrops(dropped func() uint64) {
	prometheus.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "app_radio_events_dropped_total",
		Help: "Radio events not written to the database because the write buffer was full.",
	}, func() float64 { return float64(dropped()) }))
}

func RegisterRadioGauges(radios5G func() int, subscribers5G func() (own, roamers int), radios4G func() int, subscribers4G func() (own, roamers int)) {
	radiosDesc := prometheus.NewDesc(
		"app_connected_radios",
		"Number of radios currently connected to Ella Core, by RAT (5G gNBs, 4G eNBs).",
		[]string{"rat"},
		nil,
	)

	subscribersDesc := prometheus.NewDesc(
		"app_registered_subscribers",
		"Number of subscribers currently registered in Ella Core, by RAT (5GS, EPS) and whether the subscriber is a roamer.",
		[]string{"rat", "roaming"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(countOrZero(radios5G)), RAT5G)

		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(countOrZero(radios4G)), RAT4G)

		for rat, count := range map[string]func() (int, int){RAT5G: subscribers5G, RAT4G: subscribers4G} {
			own, roamers := splitOrZero(count)

			ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(own), rat, RoamingFalse)

			ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(roamers), rat, RoamingTrue)
		}
	}))
}

func splitOrZero(f func() (int, int)) (int, int) {
	if f == nil {
		return 0, 0
	}

	return f()
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
func RegistrationAttempt(rat, regType, result string, roaming bool) {
	if registrationAttempts == nil {
		return
	}

	registrationAttempts.WithLabelValues(rat, regType, result, RoamingLabel(roaming)).Inc()
}

func RoamingLabel(roaming bool) string {
	if roaming {
		return RoamingTrue
	}

	return RoamingFalse
}

func PeerRequest(iface, result string, elapsed time.Duration) {
	if peerRequests == nil {
		return
	}

	peerRequests.WithLabelValues(iface, result).Inc()
	peerDuration.WithLabelValues(iface).Observe(elapsed.Seconds())
}

func DroppedUsage(reason string, bytes uint64) {
	if usageBytesDropped == nil {
		return
	}

	usageBytesDropped.WithLabelValues(reason).Add(float64(bytes))
}
