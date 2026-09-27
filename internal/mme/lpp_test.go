// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestTransferLPPMsg_ConnectedUE_SendsDownlinkGenericNASTransport(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	correlationID := []byte{0x00, 0x00, 0x00, 0x07}
	lppMsg := []byte{0xd0, 0x02, 0x00, 0x00, 0x21, 0xc0}

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), correlationID, lppMsg); err != nil {
		t.Fatalf("TransferLPPMsg: %v", err)
	}

	if cc.count() != 1 {
		t.Fatalf("sent = %d, want 1", cc.count())
	}

	wire := decodeDownlinkNAS(t, cc.snapshot()[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}

	dl, err := eps.ParseDownlinkGenericNASTransport(plain)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if dl.ContainerType != eps.GenericMessageContainerTypeLPP {
		t.Errorf("container type = %s, want LPP", dl.ContainerType)
	}

	if !bytes.Equal(dl.Container, lppMsg) {
		t.Errorf("container = %x, want %x", dl.Container, lppMsg)
	}

	if !bytes.Equal(dl.AdditionalInformation, correlationID) {
		t.Errorf("additional information = %x, want %x", dl.AdditionalInformation, correlationID)
	}
}

func TestTransferLPPMsg_AssignsCorrelationID(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	m.IndexRadioForTest(&captureConn{}, []SupportedTAI{{Tai: models.Tai{PlmnID: &plmn, Tac: "000001"}}})

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), nil, []byte{0x01}); err != nil {
		t.Fatalf("TransferLPPMsg: %v", err)
	}

	defer func() {
		m.mu.Lock()
		ue.clearPaging()
		m.mu.Unlock()
	}()

	buf := ue.PopLPPBuffered()
	if buf == nil {
		t.Fatal("expected the LPP message to be buffered")
	}

	if len(buf.CorrelationID) != 4 {
		t.Errorf("correlation id = %x, want a 4-octet MME-assigned value", buf.CorrelationID)
	}
}

func TestTransferLPPMsg_IdleUE_BuffersAndPages(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	enb := &captureConn{}
	m.IndexRadioForTest(enb, []SupportedTAI{{Tai: models.Tai{PlmnID: &plmn, Tac: "000001"}}})

	correlationID := []byte{0x00, 0x00, 0x00, 0x02}
	lppMsg := []byte{0xaa, 0xbb}

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), correlationID, lppMsg); err != nil {
		t.Fatalf("TransferLPPMsg: %v", err)
	}

	defer func() {
		m.mu.Lock()
		ue.clearPaging()
		m.mu.Unlock()
	}()

	if got := enb.count(); got != 1 {
		t.Errorf("pages sent = %d, want 1", got)
	}

	if !m.pagingActive(ue) {
		t.Error("expected paging supervision to be armed")
	}

	buf := ue.PopLPPBuffered()
	if buf == nil {
		t.Fatal("expected the LPP message to be buffered")
	}

	if !bytes.Equal(buf.CorrelationID, correlationID) || !bytes.Equal(buf.Payload, lppMsg) {
		t.Errorf("buffered = %+v, want correlation %x payload %x", buf, correlationID, lppMsg)
	}
}

func TestTransferLPPMsg_RejectsUnregisteredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(EMMDeregistered)

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), nil, []byte{0x01}); err == nil {
		t.Error("expected an error for a UE outside EMM-REGISTERED")
	}

	if cc.count() != 0 {
		t.Errorf("sent = %d, want 0", cc.count())
	}
}

func TestDeliverBufferedLPP(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ue.SetLPPBuffered([]byte{0x00, 0x00, 0x00, 0x03}, []byte{0x01, 0x02})

	m.DeliverBufferedLPP(context.Background(), ue, ue.Conn())

	if cc.count() != 1 {
		t.Fatalf("sent = %d, want 1", cc.count())
	}

	if ue.PopLPPBuffered() != nil {
		t.Error("the delivered message is still buffered")
	}
}

func TestCancelBufferedLPP(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)
	supi := lppaTestSUPI(t, ue)

	ue.SetLPPBuffered([]byte{0x01}, []byte{0xaa})

	m.CancelBufferedLPP(supi, []byte{0x02})

	if ue.PopLPPBuffered() == nil {
		t.Fatal("a cancel for another session must not discard the message")
	}

	ue.SetLPPBuffered([]byte{0x01}, []byte{0xaa})

	m.CancelBufferedLPP(supi, []byte{0x01})

	if ue.PopLPPBuffered() != nil {
		t.Error("a cancel for the buffered session must discard it")
	}
}

func TestClearPagingDropsTheBufferedLPP(t *testing.T) {
	m := newTestMME(t)
	ue := idleRegisteredUE(t, m)

	ue.SetLPPBuffered([]byte{0x01}, []byte{0xaa})

	if err := m.NotifyDownlinkData(context.Background(), ue.imsiOrEmpty(), 5, models.DownlinkDataArrived); err != nil {
		t.Fatalf("Page: %v", err)
	}

	ue.PagingFailed(t.Context(), models.EPSPagingUENotResponding)

	if ue.PopLPPBuffered() != nil {
		t.Error("the buffered LPP message survived the failed paging procedure")
	}
}

func TestAbandonPaging_DiscardsBufferedLPP(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = 5 * time.Millisecond
	m.pagingCfg.MaxRetryTimes = 1

	ue := idleRegisteredUE(t, m)

	ue.SetLPPBuffered([]byte{0x01}, []byte{0xaa})

	_ = ue.beginPaging(&MTRequest{})
	m.armPaging(t.Context(), ue, []byte{0x00})

	deadline := time.Now().Add(2 * time.Second)

	for ue.lppBuf.pending() {
		if time.Now().After(deadline) {
			t.Fatal("expected the buffered LPP message discarded when paging was abandoned")
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func TestTransferLPPMsg_IdleUE_JoinsPagingInProgress(t *testing.T) {
	m := newTestMME(t)
	m.pagingCfg.ExpireTime = time.Hour

	ue := idleRegisteredUE(t, m)

	_ = ue.beginPaging(&MTRequest{})
	m.armPaging(t.Context(), ue, []byte{0x00})

	defer func() {
		m.mu.Lock()
		ue.clearPaging()
		m.mu.Unlock()
	}()

	correlationID := []byte{0x00, 0x00, 0x00, 0x09}

	if err := m.TransferLPPMsg(context.Background(), lppaTestSUPI(t, ue), correlationID, []byte{0xaa}); err != nil {
		t.Fatalf("TransferLPPMsg: %v", err)
	}

	buf := ue.PopLPPBuffered()
	if buf == nil {
		t.Fatal("expected the LPP message buffered on the paging procedure in progress")
	}

	if !bytes.Equal(buf.CorrelationID, correlationID) {
		t.Errorf("correlation id = %x, want %x", buf.CorrelationID, correlationID)
	}
}
