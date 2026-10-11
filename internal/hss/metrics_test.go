// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/hss"
	"github.com/prometheus/client_golang/prometheus"
)

func TestIMSMetrics(t *testing.T) {
	store := newFakeIMSStore()
	store.registrations[testIMSI] = hss.Registration{State: hss.Registered, ServerName: testSCSCF}
	store.registrations[testOtherIMSI] = hss.Registration{State: hss.Unregistered, ServerName: testSCSCF}

	reg := prometheus.NewRegistry()
	newHSS(store).RegisterMetrics(reg)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	if len(families) != 1 || families[0].GetName() != "app_ims_registered_subscribers" || len(families[0].GetMetric()) != 1 {
		t.Fatalf("gathered %v, want one app_ims_registered_subscribers series", families)
	}

	if got := families[0].GetMetric()[0].GetGauge().GetValue(); got != 1 {
		t.Fatalf("app_ims_registered_subscribers = %v, want 1", got)
	}
}
