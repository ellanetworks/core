// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	registrationAccepted    = "accept"
	registrationAuthFailure = "auth_failure"
	registrationRejected    = "reject"

	registeredCountTimeout = 5 * time.Second
)

var registrationAttempts *prometheus.CounterVec

func (h *HSS) RegisterMetrics() {
	registrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_ims_registration_attempts_total",
		Help: "IMS registrations and re-registrations as the HSS sees them, by result: accept (the S-CSCF assigned " +
			"the subscriber), auth_failure (the S-CSCF reported an authentication failure or timeout) and reject " +
			"(the HSS refused the subscriber).",
	}, []string{"result"})

	for _, result := range []string{registrationAccepted, registrationAuthFailure, registrationRejected} {
		registrationAttempts.WithLabelValues(result)
	}

	registered := prometheus.NewDesc(
		"app_ims_registered_subscribers",
		"Number of subscribers currently registered in IMS, across the cluster.",
		nil,
		nil,
	)

	prometheus.MustRegister(registrationAttempts, prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ctx, cancel := context.WithTimeout(context.Background(), registeredCountTimeout)
		defer cancel()

		n, err := h.store.CountRegistered(ctx)
		if err != nil {
			h.log.Warn("count IMS registrations failed", zap.Error(err))
			return
		}

		ch <- prometheus.MustNewConstMetric(registered, prometheus.GaugeValue, float64(n))
	}))
}

func recordAuthorization(req, ans *diameter.Message) {
	r, err := cx.ParseUserAuthorizationRequest(req)
	if err != nil || r.AuthorizationType == cx.AuthorizationDeregistration {
		return
	}

	if refused(ans) {
		recordRegistrationAttempt(registrationRejected)
	}
}

func recordAssignment(req, ans *diameter.Message) {
	r, err := cx.ParseServerAssignmentRequest(req)
	if err != nil {
		return
	}

	switch r.Type {
	case cx.AssignmentRegistration, cx.AssignmentReRegistration:
		if succeeded(ans) {
			recordRegistrationAttempt(registrationAccepted)
		} else if refused(ans) {
			recordRegistrationAttempt(registrationRejected)
		}
	case cx.AssignmentAuthenticationFailure, cx.AssignmentAuthenticationTimeout:
		if succeeded(ans) {
			recordRegistrationAttempt(registrationAuthFailure)
		}
	}
}

func succeeded(ans *diameter.Message) bool {
	r, err := tgpp.ParseResult(ans)
	return err == nil && !r.Failure()
}

func refused(ans *diameter.Message) bool {
	r, err := tgpp.ParseResult(ans)
	if err != nil || !r.Failure() {
		return false
	}

	return r.Experimental || r.Code < 3000 || r.Code >= 4000
}

func recordRegistrationAttempt(result string) {
	if registrationAttempts != nil {
		registrationAttempts.WithLabelValues(result).Inc()
	}
}
