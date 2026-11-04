//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type vsmfExchange struct {
	method string
	path   string
	status int
}

type vsmfRecordingN32 struct {
	*sepp.Manager

	mu  sync.Mutex
	log []vsmfExchange
}

func (r *vsmfRecordingN32) Client(partnerID string) *http.Client {
	inner := r.Manager.Client(partnerID)

	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := inner.Do(req)

		status := 0
		if err == nil {
			status = resp.StatusCode
		}

		r.mu.Lock()
		r.log = append(r.log, vsmfExchange{method: req.Method, path: req.URL.Path, status: status})
		r.mu.Unlock()

		return resp, err
	})}
}

func (r *vsmfRecordingN32) status(t *testing.T, method, suffix string) int {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.log {
		if e.method == method && strings.HasSuffix(e.path, suffix) && (suffix != "" || strings.Contains(e.path, "/sdm-subscriptions/")) {
			return e.status
		}
	}

	t.Fatalf("no %s …%s over N32-f; exchanges: %+v", method, suffix, r.log)

	return 0
}

func TestOpen5GSHPLMNUDMForVisitorSessions(t *testing.T) {
	ctx := context.Background()
	partner := open5GSPartner(t)

	m := startEllaSEPP(t, partner, ellaIdentity(t, 3), 7446)
	waitEstablished(t, m, partner.ID)

	n32 := &vsmfRecordingN32{Manager: m}

	v := NewVSMF(zap.NewExample())
	v.SetN32(n32)

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	target := smf.HPLMNTarget{PartnerID: partner.ID, HPLMN: e2eHPLMN.Model(), ServingNetwork: e2eOperator.Model(), HPLMNSnssai: models.Snssai{Sst: 1}, LBO: true}

	h, err := v.OpenSubscription(ctx, smf.HPLMNSubscriptionRequest{Supi: supi, PDUSessionID: 5, Dnn: "internet", Target: target})
	if err != nil || h == nil {
		t.Fatalf("OpenSubscription through Open5GS = %v, %v", h, err)
	}

	t.Logf("home session rules: %+v", h.Rules())

	if err := h.Register(ctx); err != nil {
		t.Fatalf("UECM SMF registration at Open5GS: %v", err)
	}

	h.Close()
	v.Settle()

	for _, c := range []struct {
		method, suffix string
		want           []int
	}{
		{http.MethodGet, "/sm-data", []int{http.StatusOK}},
		{http.MethodPost, "/sdm-subscriptions", []int{http.StatusCreated}},
		{http.MethodPut, "/smf-registrations/5", []int{http.StatusCreated, http.StatusOK}},
		{http.MethodDelete, "/smf-registrations/5", []int{http.StatusNoContent}},
		{http.MethodDelete, "", []int{http.StatusNoContent}},
	} {
		if got := n32.status(t, c.method, c.suffix); !slices.Contains(c.want, got) {
			t.Errorf("%s …%s = %d, want one of %v", c.method, c.suffix, got, c.want)
		}
	}

	n32.mu.Lock()
	defer n32.mu.Unlock()

	t.Logf("N32-f exchanges: %+v", n32.log)
}
