// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

type capacitySessionManager struct{ fakeSessionManager }

func (capacitySessionManager) CreateEPSSession(context.Context, models.EPSBearerRequest) (models.EPSBearer, error) {
	return models.EPSBearer{}, fmt.Errorf("UPF session establishment failed: %w", smf.ErrDatapathCapacity)
}

func newCapacityMME(t *testing.T) *mme.MME {
	t.Helper()

	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, &capacitySessionManager{})
	m.NAS = &nasHandler{m: m}

	return m
}

func rrcCause(c s1ap.RRCEstablishmentCause) *s1ap.RRCEstablishmentCause { return &c }

func assertESMCapacityReject(t *testing.T, reject *eps.PDNConnectivityReject, wantBackoff bool) {
	t.Helper()

	if reject.Cause != eps.ESMCauseInsufficientResources {
		t.Fatalf("ESM cause = %d, want #26", reject.Cause)
	}

	if !wantBackoff {
		if reject.BackoffTimer != nil {
			t.Fatalf("back-off timer = %+v, want none", reject.BackoffTimer)
		}

		return
	}

	if reject.BackoffTimer == nil {
		t.Fatal("expected a Back-off timer value IE")
	}

	if d, ok := reject.BackoffTimer.Duration(); !ok || d != 60*time.Second {
		t.Fatalf("back-off = %v (%t), want 60s", d, ok)
	}
}

func TestAdditionalPDNRejectedAtDatapathCapacity(t *testing.T) {
	cases := []struct {
		name        string
		cause       *s1ap.RRCEstablishmentCause
		wantBackoff bool
	}{
		{"mo-Data", rrcCause(s1ap.RRCCauseMOData), true},
		{"mo-Signalling", rrcCause(s1ap.RRCCauseMOSignalling), true},
		{"highPriorityAccess", rrcCause(s1ap.RRCCauseHighPriorityAccess), false},
		{"emergency", rrcCause(s1ap.RRCCauseEmergency), false},
		{"cause unknown", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCapacityMME(t)
			ue, cc := securedUE(t, m)
			ue.Conn().SetRRCEstablishmentCause(tc.cause)
			testPDN(ue).Apn = "internet"

			handlePDNConnectivityRequest(context.Background(), m, ue, ue.Conn(), imsConnectivityRequest(2))

			if ue.PdnForAPN("ims") != nil {
				t.Fatal("PDN created although the datapath is at capacity")
			}

			reject, err := eps.ParsePDNConnectivityReject(lastDownlinkESM(t, ue, cc))
			if err != nil {
				t.Fatalf("expected a PDN Connectivity Reject: %v", err)
			}

			assertESMCapacityReject(t, reject, tc.wantBackoff)
		})
	}
}

func TestAttachRejectedAtDatapathCapacityCarriesBackoff(t *testing.T) {
	m := newCapacityMME(t)
	ue, cc := securedUE(t, m)
	ue.Conn().SetRRCEstablishmentCause(rrcCause(s1ap.RRCCauseMOSignalling))

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	rej, err := eps.ParseAttachReject(decodeProtectedDownlink(t, ue, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseESMFailure {
		t.Fatalf("Attach Reject cause = %d, want #19", rej.Cause)
	}

	esm, err := eps.ParsePDNConnectivityReject(rej.ESMMessageContainer)
	if err != nil {
		t.Fatalf("ESM message container is not a PDN Connectivity Reject: %v", err)
	}

	assertESMCapacityReject(t, esm, true)
}

func TestCapacityBackoffRules(t *testing.T) {
	capacity := fmt.Errorf("wrapped: %w", smf.ErrDatapathCapacity)

	cases := []struct {
		name        string
		cause       *s1ap.RRCEstablishmentCause
		requestType eps.RequestType
		err         error
		want        bool
	}{
		{"initial request", rrcCause(s1ap.RRCCauseMOData), eps.RequestTypeInitialRequest, capacity, true},
		{"emergency request", rrcCause(s1ap.RRCCauseMOData), eps.RequestTypeEmergency, capacity, false},
		{"handover of emergency bearer services", rrcCause(s1ap.RRCCauseMOData), eps.RequestTypeHandoverOfEmergencyBearerServices, capacity, false},
		{"handover", rrcCause(s1ap.RRCCauseMOData), eps.RequestTypeHandover, capacity, false},
		{"high priority access", rrcCause(s1ap.RRCCauseHighPriorityAccess), eps.RequestTypeInitialRequest, capacity, false},
		{"not a capacity failure", rrcCause(s1ap.RRCCauseMOData), eps.RequestTypeInitialRequest, errors.New("no IP pool"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newCapacityMME(t)
			ue, _ := securedUE(t, m)
			ue.Conn().SetRRCEstablishmentCause(tc.cause)

			got := capacityBackoff(ue.Conn(), tc.requestType, tc.err)
			if (got != nil) != tc.want {
				t.Fatalf("back-off = %+v, want present=%t", got, tc.want)
			}
		})
	}
}

func TestAttachBearerRejectCauseMapsDatapathCapacity(t *testing.T) {
	err := fmt.Errorf("establish: %w", smf.ErrDatapathCapacity)

	if got := attachBearerRejectCause(eps.RequestTypeInitialRequest, err); got != eps.ESMCauseInsufficientResources {
		t.Fatalf("cause = %d, want #26", got)
	}
}

func TestCapacityBackoffIsTheSharedDatapathBackoff(t *testing.T) {
	m := newCapacityMME(t)
	ue, _ := securedUE(t, m)
	ue.Conn().SetRRCEstablishmentCause(rrcCause(s1ap.RRCCauseMOData))

	got := capacityBackoff(ue.Conn(), eps.RequestTypeInitialRequest, fmt.Errorf("establish: %w", models.ErrDatapathCapacity))
	if got == nil {
		t.Fatal("no back-off for a datapath capacity failure")
	}

	if d, ok := got.Duration(); !ok || d != models.DatapathCapacityBackoff {
		t.Fatalf("back-off = %v (%t), want the %v the SMF uses", d, ok, models.DatapathCapacityBackoff)
	}
}

func TestAttachBearerRejectCauseNetworkInstanceUnavailable(t *testing.T) {
	err := fmt.Errorf("establish: %w", models.ErrNetworkInstanceUnavailable)
	if got := attachBearerRejectCause(eps.RequestTypeInitialRequest, err); got != eps.ESMCauseInsufficientResources {
		t.Fatalf("cause = %d, want insufficient resources (#26)", got)
	}
}
