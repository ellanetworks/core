// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"
)

func TestHandlePagingFailure_ResetsDownlinkNotification(t *testing.T) {
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

	if got := upf.resetDDNCalls; len(got) != 1 || got[0] != 4242 {
		t.Fatalf("reset calls = %v, want [4242]", got)
	}
}

func TestHandleEPSPagingFailure_ResetsDownlinkNotification(t *testing.T) {
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

	if got := upf.resetDDNCalls; len(got) != 1 || got[0] != 7 {
		t.Fatalf("reset calls = %v, want [7]", got)
	}
}

func TestHandlePagingFailure_NoSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if err := s.HandlePagingFailure(context.Background(), testSUPI(), 1); err == nil {
		t.Fatal("expected error for missing session")
	}

	if len(upf.resetDDNCalls) != 0 {
		t.Fatalf("unexpected reset calls: %v", upf.resetDDNCalls)
	}
}
