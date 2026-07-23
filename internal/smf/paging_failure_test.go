// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"
)

// TestHandleEPSPagingFailure_SuppressesDownlinkNotification pins the invariant
// shared by TS 23.401 §5.3.4.3 (EPS) and TS 23.502 §4.2.3.3 (5GS): a failed page
// must not be re-armed by the next downlink packet. The handler keeps the session's
// downlink data-notification dedup set (suppressed), rather than clearing it, so an
// unreachable UE is not paged again until it returns and the bearer is reactivated.
func TestHandleEPSPagingFailure_SuppressesDownlinkNotification(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	supi := testSUPI()

	const ebi = 5

	smCtx := s.NewSession(supi, ebi, testDNN, testSnssai)
	smCtx.SetPFCPSession(s.AllocateLocalSEID())
	smCtx.PFCPContext.RemoteSEID = 7

	if err := s.HandleEPSPagingFailure(context.Background(), testIMSI, ebi); err != nil {
		t.Fatalf("HandleEPSPagingFailure: %v", err)
	}

	if got := upf.suppressDDNCalls; len(got) != 1 || got[0] != 7 {
		t.Fatalf("suppress calls = %v, want [7]", got)
	}
}

func TestHandlePagingFailure_SuppressesDownlinkNotification(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	supi := testSUPI()

	const pduSessionID = 1

	smCtx := s.NewSession(supi, pduSessionID, testDNN, testSnssai)
	smCtx.SetPFCPSession(s.AllocateLocalSEID())
	smCtx.PFCPContext.RemoteSEID = 4242

	if err := s.HandlePagingFailure(context.Background(), supi, pduSessionID); err != nil {
		t.Fatalf("HandlePagingFailure: %v", err)
	}

	if got := upf.suppressDDNCalls; len(got) != 1 || got[0] != 4242 {
		t.Fatalf("suppress calls = %v, want [4242]", got)
	}
}

func TestHandlePagingFailure_NoSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if err := s.HandlePagingFailure(context.Background(), testSUPI(), 1); err == nil {
		t.Fatal("expected error for missing session")
	}

	if len(upf.suppressDDNCalls) != 0 {
		t.Fatalf("unexpected suppress calls: %v", upf.suppressDDNCalls)
	}
}
