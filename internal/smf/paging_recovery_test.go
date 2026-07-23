// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"
)

func TestClearEPSPagingSuppression_ClearsDownlinkNotification(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	supi := testSUPI()

	const ebi = 5

	smCtx := s.NewSession(supi, ebi, testDNN, testSnssai)
	smCtx.SetPFCPSession(s.AllocateLocalSEID())
	smCtx.PFCPContext.RemoteSEID = 7

	if err := s.ClearEPSPagingSuppression(context.Background(), testIMSI, ebi); err != nil {
		t.Fatalf("ClearEPSPagingSuppression: %v", err)
	}

	if got := upf.clearDDNCalls; len(got) != 1 || got[0] != 7 {
		t.Fatalf("clear calls = %v, want [7]", got)
	}
}

func TestClearPagingSuppression_ClearsDownlinkNotification(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	supi := testSUPI()

	const pduSessionID = 1

	smCtx := s.NewSession(supi, pduSessionID, testDNN, testSnssai)
	smCtx.SetPFCPSession(s.AllocateLocalSEID())
	smCtx.PFCPContext.RemoteSEID = 4242

	if err := s.ClearPagingSuppression(context.Background(), supi, pduSessionID); err != nil {
		t.Fatalf("ClearPagingSuppression: %v", err)
	}

	if got := upf.clearDDNCalls; len(got) != 1 || got[0] != 4242 {
		t.Fatalf("clear calls = %v, want [4242]", got)
	}
}

func TestClearPagingSuppression_NoSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	if err := s.ClearPagingSuppression(context.Background(), testSUPI(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(upf.clearDDNCalls) != 0 {
		t.Fatalf("unexpected clear calls: %v", upf.clearDDNCalls)
	}
}
