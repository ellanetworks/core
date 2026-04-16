// Copyright 2026 Ella Networks

package ngap_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
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

func TestHandleInitialUEMessage_CreatesNewRanUe(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	nasPDU := []byte{0xAA, 0xBB, 0xCC}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
	})

	if len(ran.RanUEs) != 1 {
		t.Fatalf("len(ran.RanUEs) = %d, want 1", len(ran.RanUEs))
	}

	ranUe := ran.RanUEs[1]
	if ranUe == nil {
		t.Fatal("ran.RanUEs[1] is nil")
	}

	if ranUe.RanUeNgapID != 1 {
		t.Errorf("RanUeNgapID = %d, want 1", ranUe.RanUeNgapID)
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}
}

func TestHandleInitialUEMessage_ReusedRanUeNgapID_EvictsStale(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	amf.NewRanUeForTest(ran, 1, 99, logger.AmfLog)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
	})

	if len(ran.RanUEs) != 1 {
		t.Fatalf("len(ran.RanUEs) = %d, want 1", len(ran.RanUEs))
	}

	newRanUe := ran.RanUEs[1]
	if newRanUe == nil {
		t.Fatal("ran.RanUEs[1] is nil")
	}

	if newRanUe.AmfUeNgapID == 99 {
		t.Error("stale RanUe was not evicted — same AmfUeNgapID")
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}

func TestHandleInitialUEMessage_5GSTMSI_KnownUE_AttachesAmfUe(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &FakeDBInstance{
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

	guti, err := etsi.NewGUTI("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatal(err)
	}

	amfUe := amf.NewAmfUe()
	amfUe.Guti = guti

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	amfUe.Supi = supi
	amfUe.Log = logger.AmfLog

	err = amfInstance.AddAmfUeToUePool(amfUe)
	if err != nil {
		t.Fatal(err)
	}

	ran := newTestRadio()
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

	ranUe := ran.RanUEs[1]
	if ranUe == nil {
		t.Fatal("ran.RanUEs[1] is nil")
	}

	if ranUe.AmfUe() != amfUe {
		t.Error("ranUe.AmfUe() does not point to the expected AmfUe")
	}

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}

func TestHandleInitialUEMessage_5GSTMSI_UnknownUE_NASStillCalled(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &FakeDBInstance{
		Operator: &db.Operator{
			Mcc:         "001",
			Mnc:         "01",
			AmfRegionID: 0xCA,
			AmfSetID:    0x3F8,
		},
	}

	ran := newTestRadio()
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

	ranUe := ran.RanUEs[1]
	if ranUe == nil {
		t.Fatal("ran.RanUEs[1] is nil")
	}

	if ranUe.AmfUe() != nil {
		t.Error("ranUe.AmfUe() should be nil for unknown UE")
	}
}

func TestHandleInitialUEMessage_SetsUeContextRequest(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID:      1,
		NASPDU:           []byte{0x01},
		UEContextRequest: true,
	})

	ranUe := ran.RanUEs[1]
	if ranUe == nil {
		t.Fatal("ran.RanUEs[1] is nil")
	}

	if !ranUe.UeContextRequest {
		t.Error("UeContextRequest = false, want true")
	}
}

func TestHandleInitialUEMessage_RegisteredUE_DoesNotPanic(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	amfInstance.DBInstance = &FakeDBInstance{
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

	guti, err := etsi.NewGUTI("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatal(err)
	}

	amfUe := amf.NewAmfUe()
	amfUe.Guti = guti

	supi, err := etsi.NewSUPIFromIMSI("001010000000002")
	if err != nil {
		t.Fatal(err)
	}

	amfUe.Supi = supi
	amfUe.Log = logger.AmfLog

	err = amfInstance.AddAmfUeToUePool(amfUe)
	if err != nil {
		t.Fatal(err)
	}

	ran := newTestRadio()
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

func TestHandleInitialUEMessage_NASReturnsError(t *testing.T) {
	fakeNAS := &FakeNASHandler{Err: errors.New("nas decode failed")}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "001"}}

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x01},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	// Verify no downstream side effects: ran still has the UE
	if ran.RanUEs[1] == nil {
		t.Error("RanUe should still exist after NAS error")
	}
}
