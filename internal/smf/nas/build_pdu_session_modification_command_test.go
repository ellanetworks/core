// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestBuildPDUSessionModificationCommand_AmbrAndQoS(t *testing.T) {
	ambr := &models.Ambr{
		Uplink:   "200 Mbps",
		Downlink: "200 Mbps",
	}
	qosData := &models.QosData{
		QFI:    1,
		Var5qi: 8,
		Arp:    &models.Arp{PriorityLevel: 14},
	}

	encoded, err := smfNas.BuildPDUSessionModificationCommand(1, ambr, qosData)
	if err != nil {
		t.Fatalf("BuildPDUSessionModificationCommand failed: %v", err)
	}

	// Decode and verify round-trip.
	m := new(nas.Message)
	if err := m.PlainNasDecode(&encoded); err != nil {
		t.Fatalf("PlainNasDecode failed: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionModificationCommand {
		t.Fatalf("unexpected message type: got %d, want %d", m.GsmHeader.GetMessageType(), nas.MsgTypePDUSessionModificationCommand)
	}

	modCmd := m.PDUSessionModificationCommand
	if modCmd == nil {
		t.Fatal("PDUSessionModificationCommand is nil after decode")
	}

	// Verify SessionAMBR is present with correct IEI.
	if modCmd.SessionAMBR == nil {
		t.Fatal("SessionAMBR is nil after decode; IEI likely missing or incorrect")
	}

	if modCmd.SessionAMBR.GetIei() != nasMessage.PDUSessionModificationCommandSessionAMBRType {
		t.Fatalf("SessionAMBR IEI: got 0x%02x, want 0x%02x",
			modCmd.SessionAMBR.GetIei(), nasMessage.PDUSessionModificationCommandSessionAMBRType)
	}

	// Verify AMBR values: 200 Mbps = unit Mbps (0x06), value 200.
	ulUnit := modCmd.GetUnitForSessionAMBRForUplink()
	if ulUnit != nasMessage.SessionAMBRUnit1Mbps {
		t.Errorf("uplink AMBR unit: got %d, want %d", ulUnit, nasMessage.SessionAMBRUnit1Mbps)
	}

	dlUnit := modCmd.GetUnitForSessionAMBRForDownlink()
	if dlUnit != nasMessage.SessionAMBRUnit1Mbps {
		t.Errorf("downlink AMBR unit: got %d, want %d", dlUnit, nasMessage.SessionAMBRUnit1Mbps)
	}

	// Verify QoS flow descriptions are present.
	if modCmd.AuthorizedQosFlowDescriptions == nil {
		t.Fatal("AuthorizedQosFlowDescriptions is nil after decode")
	}
}

func TestBuildPDUSessionModificationCommand_AmbrOnly(t *testing.T) {
	ambr := &models.Ambr{
		Uplink:   "300 Mbps",
		Downlink: "400 Mbps",
	}

	encoded, err := smfNas.BuildPDUSessionModificationCommand(5, ambr, nil)
	if err != nil {
		t.Fatalf("BuildPDUSessionModificationCommand failed: %v", err)
	}

	m := new(nas.Message)
	if err := m.PlainNasDecode(&encoded); err != nil {
		t.Fatalf("PlainNasDecode failed: %v", err)
	}

	modCmd := m.PDUSessionModificationCommand
	if modCmd == nil {
		t.Fatal("PDUSessionModificationCommand is nil")
	}

	if modCmd.SessionAMBR == nil {
		t.Fatal("SessionAMBR is nil; expected AMBR-only modification to include it")
	}

	if modCmd.AuthorizedQosFlowDescriptions != nil {
		t.Fatal("AuthorizedQosFlowDescriptions should be nil for AMBR-only modification")
	}
}

func TestBuildPDUSessionModificationCommand_QoSOnly(t *testing.T) {
	qosData := &models.QosData{
		QFI:    1,
		Var5qi: 7,
		Arp:    &models.Arp{PriorityLevel: 10},
	}

	encoded, err := smfNas.BuildPDUSessionModificationCommand(3, nil, qosData)
	if err != nil {
		t.Fatalf("BuildPDUSessionModificationCommand failed: %v", err)
	}

	m := new(nas.Message)
	if err := m.PlainNasDecode(&encoded); err != nil {
		t.Fatalf("PlainNasDecode failed: %v", err)
	}

	modCmd := m.PDUSessionModificationCommand
	if modCmd == nil {
		t.Fatal("PDUSessionModificationCommand is nil")
	}

	if modCmd.SessionAMBR != nil {
		t.Fatal("SessionAMBR should be nil for QoS-only modification")
	}

	if modCmd.AuthorizedQosFlowDescriptions == nil {
		t.Fatal("AuthorizedQosFlowDescriptions is nil; expected QoS-only modification to include it")
	}
}
