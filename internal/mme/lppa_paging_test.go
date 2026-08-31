// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

func lppaTestSUPI(t *testing.T, ue *UeContext) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(ue.imsiOrEmpty())
	if err != nil {
		t.Fatalf("build SUPI: %v", err)
	}

	return supi
}

// TS 23.273 §6.11.2
func TestPageAndRetryLPPa_IdleUE_BuffersAndPages(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)
	supi := lppaTestSUPI(t, ue)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	enb := &captureConn{}
	m.IndexRadioForTest(enb, []SupportedTAI{{Tai: models.Tai{PlmnID: &plmn, Tac: "000001"}}})

	payload := []byte{0xaa, 0xbb, 0xcc}

	if err := m.PageAndRetryLPPa(context.Background(), supi, 4, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := enb.count(); got != 1 {
		t.Errorf("pages sent = %d, want 1", got)
	}

	if !m.pagingActive(ue) {
		t.Error("expected paging supervision to be armed")
	}

	buf := ue.PopLPPaBuffered()
	if buf == nil {
		t.Fatal("expected the LPPa payload to be buffered")
	}

	if buf.MeasurementID != 4 {
		t.Errorf("MeasurementID = %d, want 4", buf.MeasurementID)
	}

	if !bytes.Equal(buf.Payload, payload) {
		t.Errorf("Payload = %x, want %x", buf.Payload, payload)
	}

	m.mu.Lock()
	m.stopPagingLocked(ue)
	m.mu.Unlock()
}

func TestPageAndRetryLPPa_RejectsUEThatNeedsNoPage(t *testing.T) {
	t.Run("already connected", func(t *testing.T) {
		m := newTestMME(t)
		ue, _ := securedUE(t, m)

		if err := m.PageAndRetryLPPa(context.Background(), lppaTestSUPI(t, ue), 1, []byte{0x01}); err == nil {
			t.Error("expected an error for an ECM-CONNECTED UE")
		}

		if ue.PopLPPaBuffered() != nil {
			t.Error("a rejected request must not leave a payload buffered")
		}
	})

	t.Run("paging already in progress", func(t *testing.T) {
		m := newTestMME(t)
		m.pagingCfg.ExpireTime = time.Hour

		ue := idleRegisteredUE(t, m)

		m.armPaging(ue, []byte{0x00})

		defer func() {
			m.mu.Lock()
			m.stopPagingLocked(ue)
			m.mu.Unlock()
		}()

		if err := m.PageAndRetryLPPa(context.Background(), lppaTestSUPI(t, ue), 1, []byte{0x01}); err == nil {
			t.Error("expected an error when a paging procedure is already in progress")
		}

		if ue.PopLPPaBuffered() != nil {
			t.Error("a rejected request must not leave a payload buffered")
		}
	})

	t.Run("unknown UE", func(t *testing.T) {
		m := newTestMME(t)

		supi, err := etsi.NewSUPIFromIMSI("001010000000999")
		if err != nil {
			t.Fatal(err)
		}

		if err := m.PageAndRetryLPPa(context.Background(), supi, 1, []byte{0x01}); err == nil {
			t.Error("expected an error for an unknown UE")
		}
	})
}

func TestCancelBufferedLPPa(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	supi := lppaTestSUPI(t, ue)

	ue.SetLPPaBuffered(9, []byte{0x01})

	m.CancelBufferedLPPa(supi, 8)

	if ue.PopLPPaBuffered() == nil {
		t.Fatal("a cancel for another measurement must not discard the payload")
	}

	ue.SetLPPaBuffered(9, []byte{0x01})

	m.CancelBufferedLPPa(supi, 9)

	if ue.PopLPPaBuffered() != nil {
		t.Error("a cancel for the buffered measurement must discard it")
	}
}

func lppaBuffered(ue *UeContext) bool {
	ue.lppaBufMu.RLock()
	defer ue.lppaBufMu.RUnlock()

	return ue.lppaBuf != nil
}

func TestAbandonPaging_DiscardsBufferedLPPa(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = 5 * time.Millisecond
	m.pagingCfg.MaxRetryTimes = 1

	ue := idleRegisteredUE(t, m)

	ue.SetLPPaBuffered(3, []byte{0x01})

	m.armPaging(ue, []byte{0x00})

	deadline := time.Now().Add(2 * time.Second)
	for lppaBuffered(ue) {
		if time.Now().After(deadline) {
			t.Fatal("expected the buffered payload discarded when paging was abandoned")
		}

		time.Sleep(5 * time.Millisecond)
	}

	if m.pagingActive(ue) {
		t.Error("expected paging supervision to have stopped")
	}
}
