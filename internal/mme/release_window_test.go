// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func releasingRegisteredUE(t *testing.T, m *MME) (*UeContext, *UeConn, *captureConn, *captureConn) {
	t.Helper()

	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	enb := &captureConn{}
	m.IndexRadioForTest(enb, []SupportedTAI{{Tai: models.Tai{PlmnID: &plmn, Tac: "000001"}}})

	link := &captureConn{}
	m.AttachUeConn(t.Context(), ue, m.NewUeConn(link, 8))

	conn := ue.Conn()
	m.ReleaseUEContext(context.Background(), ue, conn, CauseNASNormalRelease)

	if link.count() != 1 {
		t.Fatalf("sent = %d, want 1 UE Context Release Command", link.count())
	}

	t.Cleanup(func() {
		m.mu.Lock()
		ue.clearPaging()
		m.mu.Unlock()
	})

	return ue, conn, link, enb
}

func TestTransferLPPMsg_DuringRelease_PagesOnceReleased(t *testing.T) {
	m := newTestMME(t)
	ue, conn, link, enb := releasingRegisteredUE(t, m)

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), []byte{0x00, 0x00, 0x00, 0x02}, []byte{0xaa}); err != nil {
		t.Fatalf("TransferLPPMsg: %v", err)
	}

	if link.count() != 1 {
		t.Fatalf("sent = %d on the connection being released, want only the UE Context Release Command", link.count())
	}

	if enb.count() != 0 {
		t.Fatalf("pages sent = %d before the release completed, want 0", enb.count())
	}

	m.CompleteUEContextRelease(context.Background(), ue, conn)

	if enb.count() != 1 {
		t.Fatalf("pages sent = %d once the release completed, want 1", enb.count())
	}

	if ue.PopLPPBuffered() == nil {
		t.Fatal("expected the LPP message buffered for delivery once the UE answers")
	}
}

func TestHoldLPPaUntilReleased_PagesOnceReleased(t *testing.T) {
	m := newTestMME(t)
	ue, conn, link, enb := releasingRegisteredUE(t, m)

	if !m.HoldLPPaUntilReleased(lppaTestSUPI(t, ue), 3, []byte{0x01}) {
		t.Fatal("expected the LPPa message held on a connection being released")
	}

	if link.count() != 1 || enb.count() != 0 {
		t.Fatalf("sent = %d, pages = %d before the release completed, want 1 and 0", link.count(), enb.count())
	}

	m.CompleteUEContextRelease(context.Background(), ue, conn)

	if enb.count() != 1 {
		t.Fatalf("pages sent = %d once the release completed, want 1", enb.count())
	}

	if buf := ue.PopLPPaBuffered(); buf == nil || buf.MeasurementID != 3 {
		t.Fatalf("buffered LPPa = %+v, want measurement 3", buf)
	}
}

func TestHoldLPPaUntilReleased_NotHeldOnALiveConnection(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	if m.HoldLPPaUntilReleased(lppaTestSUPI(t, ue), 1, []byte{0x01}) {
		t.Fatal("a live connection must carry the LPPa message directly")
	}
}
