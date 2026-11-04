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

	hplmnSBIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_hplmn_sbi_requests_total",
		Help: "5G requests sent to the home network of a roaming partner, by partner, service and result (HTTP status or error).",
	}, []string{"partner", "service", "result"})

	rejects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_rejects_total",
		Help: "Registrations (5G) and attaches (4G) of roamers rejected, by partner, RAT and NAS cause.",
	}, []string{"partner", "rat", "cause"})

	hplmnTransient = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_roaming_hplmn_transient_failures_total",
		Help: "5G home network failures that carried no subscriber verdict and were treated as transient, by partner, operation, HTTP status and cause.",
	}, []string{"partner", "operation", "status", "cause"})
)

func RegisterMetrics(reg prometheus.Registerer) {
	reg.MustRegister(s6aRequests, hplmnSBIRequests, rejects, hplmnTransient)
}

func RecordHPLMNTransient(partner, operation, status, cause string) {
	hplmnTransient.WithLabelValues(partnerLabel(partner), operation, status, cause).Inc()
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

func recordHPLMNSBI(partner string, service string, status int, err error) {
	result := strconv.Itoa(status)
	if err != nil || status == 0 {
		result = resultError
	}

	hplmnSBIRequests.WithLabelValues(partnerLabel(partner), service, result).Inc()
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
