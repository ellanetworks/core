// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"testing"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/hss"
	"github.com/prometheus/client_golang/prometheus"
)

func TestIMSMetrics(t *testing.T) {
	store := newFakeIMSStore()
	store.registrations[testIMSI] = hss.Registration{State: hss.Registered, ServerName: testSCSCF}
	store.registrations[testOtherIMSI] = hss.Registration{State: hss.Unregistered, ServerName: testSCSCF}

	newHSS(store).RegisterMetrics()

	userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN})
	userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testNoIMS), PublicIdentity: "tel:+" + testNoIMSTel})
	userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN, AuthorizationType: cx.AuthorizationDeregistration})

	serverAssignment(t, store, register(testIMSI, cx.AssignmentReRegistration))
	serverAssignment(t, store, register(testOtherIMSI, cx.AssignmentAuthenticationFailure))
	serverAssignment(t, store, register(testIMSI, cx.AssignmentUserDeregistration))

	store.unavailable = true
	serverAssignment(t, store, register(testOtherIMSI, cx.AssignmentRegistration))

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	values := map[string]float64{}

	for _, f := range families {
		for _, m := range f.GetMetric() {
			name := f.GetName()
			for _, l := range m.GetLabel() {
				name += "/" + l.GetValue()
			}

			switch {
			case m.GetCounter() != nil:
				values[name] = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				values[name] = m.GetGauge().GetValue()
			}
		}
	}

	want := map[string]float64{
		"app_ims_registered_subscribers":                   0,
		"app_ims_registration_attempts_total/accept":       1,
		"app_ims_registration_attempts_total/auth_failure": 1,
		"app_ims_registration_attempts_total/reject":       1,
	}

	for name, v := range want {
		if values[name] != v {
			t.Errorf("%s = %v, want %v", name, values[name], v)
		}
	}
}
