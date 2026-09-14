// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func errorIndicationReport(seid uint64) *models.ErrorIndicationReport {
	return &models.ErrorIndicationReport{
		SEID:  seid,
		FARID: 2,
		RemoteFTEID: models.FTEID{
			TEID: 0x1234,
			Addr: netip.MustParseAddr("10.0.0.1"),
		},
	}
}

// TS 29.244 §5.10: the UP function is access agnostic, so a 5GS session and an
// EPS session are reported the same way.
func TestHandleErrorIndicationReport_ResolvesA5GSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, _ := setupSessionWithTunnel(t, s)

	if err := s.HandleErrorIndicationReport(context.Background(), errorIndicationReport(smCtx.PFCPContext.SEID)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}
}

func TestHandleErrorIndicationReport_ResolvesAnEPSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx := establishEPSForArrival(t, s)

	if err := s.HandleErrorIndicationReport(context.Background(), errorIndicationReport(smCtx.PFCPContext.SEID)); err != nil {
		t.Fatalf("an EPS session was not resolved from an Error Indication Report: %v", err)
	}
}

func TestHandleErrorIndicationReport_RejectsAnUnknownSEID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if err := s.HandleErrorIndicationReport(context.Background(), errorIndicationReport(0xdeadbeef)); err == nil {
		t.Fatal("an Error Indication for an unknown SEID was accepted")
	}
}
