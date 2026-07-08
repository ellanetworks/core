// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
)

func TestHandleInitialUEMessage_CreatesNewUeConn(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	nasPDU := []byte{0xAA, 0xBB, 0xCC}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
	})

	if ran.NumUEsForTest() != 1 {
		t.Fatalf("ran.NumUEsForTest() = %d, want 1", ran.NumUEsForTest())
	}

	ueConn := amfInstance.FindUEByRanUeNgapID(ran, 1)
	if ueConn == nil {
		t.Fatal("FindUEByRanUeNgapID(1) is nil")
	}

	if ueConn.RanUeNgapID != 1 {
		t.Errorf("RanUeNgapID = %d, want 1", ueConn.RanUeNgapID)
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}
}

// A SERVICE REQUEST Initial UE Message is routed to the dedicated HandleServiceRequest,
// never through the generic HandleNAS mint path (mirrors the MME's S1AP peek); a request
// that binds no context leaves no bare RAN connection behind.
func TestHandleInitialUEMessage_ServiceRequestRoutedToDedicatedHandler(t *testing.T) {
	fakeNAS := &fakeNASHandler{ServiceRequest: true, LeavesBare: true}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	nasPDU := []byte{0x7e, 0x00, 0x4c}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
	})

	if len(fakeNAS.ServiceRequestCalls) != 1 {
		t.Fatalf("HandleServiceRequest calls = %d, want 1", len(fakeNAS.ServiceRequestCalls))
	}

	if len(fakeNAS.Calls) != 0 {
		t.Fatalf("HandleNAS calls = %d, want 0 (a service request must not go through the mint gate)", len(fakeNAS.Calls))
	}

	if ran.NumUEsForTest() != 0 {
		t.Fatalf("bare service-request conn not released: NumUEsForTest = %d, want 0", ran.NumUEsForTest())
	}
}

// An Initial UE Message whose NAS never resolves to a UE context (undecodable, no
// usable identity) must not leave a bare RAN connection behind, or an unauthenticated
// peer could exhaust RAN-UE-NGAP-IDs.
func TestHandleInitialUEMessage_UnresolvedNAS_ReleasesBareConn(t *testing.T) {
	fakeNAS := &fakeNASHandler{LeavesBare: true}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0xAA, 0xBB, 0xCC},
	})

	if ueConn := amfInstance.FindUEByRanUeNgapID(ran, 1); ueConn != nil {
		t.Fatal("bare UeConn was not released after an unresolved NAS message")
	}

	if ran.NumUEsForTest() != 0 {
		t.Fatalf("ran.NumUEsForTest() = %d, want 0 (bare conn leaked)", ran.NumUEsForTest())
	}
}

func TestHandleInitialUEMessage_ReusedRanUeNgapID_EvictsStale(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	amf.NewUeConnForTest(ran, 1, 99, logger.AmfLog)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
	})

	if ran.NumUEsForTest() != 1 {
		t.Fatalf("ran.NumUEsForTest() = %d, want 1", ran.NumUEsForTest())
	}

	newUeConn := amfInstance.FindUEByRanUeNgapID(ran, 1)
	if newUeConn == nil {
		t.Fatal("FindUEByRanUeNgapID(1) is nil")
	}

	if newUeConn.AmfUeNgapID == 99 {
		t.Error("stale UeConn was not evicted — same AmfUeNgapID")
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}

// TestHandleInitialUEMessage_5GSTMSI_UnverifiedDoesNotAttach asserts TS 24.501:
// an Initial UE Message that resolves to a known UE by 5G-S-TMSI but
// is not integrity-verified against that context must not bind to it. The
// message is still forwarded to NAS, which processes it on a fresh context.
func TestHandleInitialUEMessage_5GSTMSI_UnverifiedDoesNotAttach(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:         "001",
			Mnc:         "01",
			AmfRegionID: 0xCA,
			AmfSetID:    0x3F8,
		},
	}

	tmsi, err := etsi.NewTMSI(0x00000001)
	if err != nil {
		t.Fatal(err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatal(err)
	}

	amfUe := amf.NewUeContext()
	amfInstance.AssignGutiForTest(amfUe, guti)

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	amfUe.SetSupiForTest(supi)

	err = amfInstance.CommitUEIdentity(context.Background(), amfUe, amf.MintAuthProofForRegistrationCommit())
	if err != nil {
		t.Fatal(err)
	}

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	tmsiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(tmsiBytes, 0x00000001)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
		FiveGSTMSI: &decode.FiveGSTMSI{
			AMFSetID:   aper.BitString{Bytes: []byte{0xFE, 0x00}, BitLength: 10},
			AMFPointer: aper.BitString{Bytes: []byte{0x00}, BitLength: 6},
			FiveGTMSI:  tmsiBytes,
		},
	})

	ueConn := amfInstance.FindUEByRanUeNgapID(ran, 1)
	if ueConn == nil {
		t.Fatal("FindUEByRanUeNgapID(1) is nil")
	}

	if ueConn.UeContext() == amfUe {
		t.Error("an unverified Initial UE Message must not bind to the existing context (TS 24.501)")
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}

func TestHandleInitialUEMessage_5GSTMSI_UnknownUE_NASStillCalled(t *testing.T) {
	fakeNAS := &fakeNASHandler{LeavesBare: true}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:         "001",
			Mnc:         "01",
			AmfRegionID: 0xCA,
			AmfSetID:    0x3F8,
		},
	}

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	tmsiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(tmsiBytes, 0x00FFFFFF)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
		FiveGSTMSI: &decode.FiveGSTMSI{
			AMFSetID:   aper.BitString{Bytes: []byte{0xFE, 0x00}, BitLength: 10},
			AMFPointer: aper.BitString{Bytes: []byte{0x00}, BitLength: 6},
			FiveGTMSI:  tmsiBytes,
		},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	// The 5G-S-TMSI resolved to no UE and the NAS body established no context, so the
	// bare connection is released (mirrors the MME's bare-connection release).
	if amfInstance.FindUEByRanUeNgapID(ran, 1) != nil {
		t.Error("bare UeConn was not released after an unknown-UE message established no context")
	}
}

func TestHandleInitialUEMessage_SetsUeContextRequest(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID:      1,
		NASPDU:           []byte{0x01},
		UEContextRequest: true,
	})

	ueConn := amfInstance.FindUEByRanUeNgapID(ran, 1)
	if ueConn == nil {
		t.Fatal("FindUEByRanUeNgapID(1) is nil")
	}

	if !ueConn.UeContextRequest {
		t.Error("UeContextRequest = false, want true")
	}
}

func TestHandleInitialUEMessage_RegisteredUE_DoesNotPanic(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:         "001",
			Mnc:         "01",
			AmfRegionID: 0xCA,
			AmfSetID:    0x3F8,
		},
	}

	tmsi, err := etsi.NewTMSI(0x00000002)
	if err != nil {
		t.Fatal(err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatal(err)
	}

	amfUe := amf.NewUeContext()
	amfInstance.AssignGutiForTest(amfUe, guti)

	supi, err := etsi.NewSUPIFromIMSI("001010000000002")
	if err != nil {
		t.Fatal(err)
	}

	amfUe.SetSupiForTest(supi)

	err = amfInstance.CommitUEIdentity(context.Background(), amfUe, amf.MintAuthProofForRegistrationCommit())
	if err != nil {
		t.Fatal(err)
	}

	ran := newTestRadio(amfInstance)
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	tmsiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(tmsiBytes, 0x00000002)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
		FiveGSTMSI: &decode.FiveGSTMSI{
			AMFSetID:   aper.BitString{Bytes: []byte{0xFE, 0x00}, BitLength: 10},
			AMFPointer: aper.BitString{Bytes: []byte{0x00}, BitLength: 6},
			FiveGTMSI:  tmsiBytes,
		},
	})

	// Timer stop methods are safe to call even when timers are nil.
	// The test verifies the handler reaches that code without panicking.
	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}
