// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"strconv"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsRAT4G = "4g"
	metricsRAT5G = "5g"

	UnknownPartner = "unknown"

	resultError   = "error"
	resultTimeout = "timeout"
)

var (
	s6aRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_s6a_requests_total",
		Help: "S6a requests exchanged with the HSS of a roaming partner, by partner, command (AIR, ULR, PUR, NOR sent; CLR, IDR, DSR, RSR received) and result (Diameter result code, timeout or error).",
	}, []string{"partner", "command", "result"})

	homeSBIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_home_sbi_requests_total",
		Help: "5G requests sent to the home network of a roaming partner, by partner, service and result (HTTP status or error).",
	}, []string{"partner", "service", "result"})

	rejects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_rejects_total",
		Help: "Registrations (5G) and attaches (4G) of roamers rejected, by partner, RAT and NAS cause.",
	}, []string{"partner", "rat", "cause"})

	visitorsDesc = prometheus.NewDesc(
		"app_roaming_visitors",
		"Roamers registered on this node, by partner and RAT.",
		[]string{"partner", "rat"},
		nil,
	)

	sessionsDesc = prometheus.NewDesc(
		"app_roaming_sessions",
		"Active sessions of roamers on this node (5G PDU sessions, 4G PDN connections), by partner and RAT.",
		[]string{"partner", "rat"},
		nil,
	)
)

type LoadSource func() map[string]PartnerLoad

func RegisterMetrics(reg prometheus.Registerer, load5G, load4G LoadSource) {
	reg.MustRegister(s6aRequests, homeSBIRequests, rejects, &loadCollector{sources: map[string]LoadSource{metricsRAT5G: load5G, metricsRAT4G: load4G}})
}

type loadCollector struct {
	sources map[string]LoadSource
}

func (c *loadCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- visitorsDesc

	ch <- sessionsDesc
}

func (c *loadCollector) Collect(ch chan<- prometheus.Metric) {
	for rat, src := range c.sources {
		if src == nil {
			continue
		}

		for partner, load := range src() {
			ch <- prometheus.MustNewConstMetric(visitorsDesc, prometheus.GaugeValue, float64(load.Roamers), partner, rat)

			ch <- prometheus.MustNewConstMetric(sessionsDesc, prometheus.GaugeValue, float64(load.Sessions), partner, rat)
		}
	}
}

func partnerLabel(name string) string {
	if name == "" {
		return UnknownPartner
	}

	return name
}

func RecordReject5G(partner, cause string) {
	rejects.WithLabelValues(partnerLabel(partner), metricsRAT5G, cause).Inc()
}

func RecordReject4G(partner, cause string) {
	rejects.WithLabelValues(partnerLabel(partner), metricsRAT4G, cause).Inc()
}

func RecordS6aAnswer(partner string, command uint32, ans *diameter.Message, err error) {
	s6aRequests.WithLabelValues(partnerLabel(partner), S6aCommandName(command), diameterResult(ans, err)).Inc()
}

func recordHomeSBI(partner string, service string, status int, err error) {
	result := strconv.Itoa(status)
	if err != nil || status == 0 {
		result = resultError
	}

	homeSBIRequests.WithLabelValues(partnerLabel(partner), service, result).Inc()
}

func diameterResult(ans *diameter.Message, err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return resultTimeout
	case err != nil || ans == nil:
		return resultError
	}

	r, perr := tgpp.ParseResult(ans)
	if perr != nil {
		return resultError
	}

	return strconv.FormatUint(uint64(r.Code), 10)
}

func S6aCommandName(command uint32) string {
	switch command {
	case s6a.CommandUpdateLocation:
		return "ULR"
	case s6a.CommandCancelLocation:
		return "CLR"
	case s6a.CommandAuthenticationInformation:
		return "AIR"
	case s6a.CommandInsertSubscriberData:
		return "IDR"
	case s6a.CommandDeleteSubscriberData:
		return "DSR"
	case s6a.CommandPurgeUE:
		return "PUR"
	case s6a.CommandReset:
		return "RSR"
	case s6a.CommandNotify:
		return "NOR"
	default:
		return strconv.FormatUint(uint64(command), 10)
	}
}
