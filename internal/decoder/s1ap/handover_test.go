// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"strings"
	"testing"

	lib "github.com/ellanetworks/core/s1ap"
)

func TestDecodeHandoverRequired(t *testing.T) {
	b, err := (&lib.HandoverRequired{
		MMEUES1APID:  7,
		ENBUES1APID:  2,
		HandoverType: lib.HandoverTypeIntraLTE,
		Cause:        lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 16},
		TargetID: lib.TargetID{TargeteNBID: lib.TargeteNBID{
			GlobalENBID: lib.GlobalENBID{PLMNIdentity: lib.PLMNIdentity{0x00, 0xf1, 0x10}, ENBID: lib.ENBID{Kind: lib.ENBIDMacro, Value: 2}},
			SelectedTAI: lib.TAI{PLMNIdentity: lib.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		}},
		SourceToTarget: lib.TransparentContainer{0xab, 0xcd},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(b)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	if !strings.HasPrefix(msg.Summary, "Handover Required") {
		t.Fatalf("summary = %q", msg.Summary)
	}

	mustIE(t, msg, idHandoverType)

	if _, ok := findIE(msg.Value.IEs, idTargetID); !ok {
		t.Fatal("TargetID IE missing")
	}

	if _, ok := findIE(msg.Value.IEs, idSourceToTargetContainer); !ok {
		t.Fatal("Source-to-Target container IE missing")
	}
}

func TestDecodeHandoverRequestAcknowledge(t *testing.T) {
	b, err := (&lib.HandoverRequestAcknowledge{
		MMEUES1APID: 7,
		ENBUES1APID: 55,
		ERABAdmitted: []lib.ERABAdmittedItem{{
			ERABID:                5,
			TransportLayerAddress: lib.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []lib.ERABItem{{ERABID: 6, Cause: lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    lib.TransparentContainer{0x11},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(b)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	admittedIE := mustIE(t, msg, idERABAdmittedList)

	admitted, ok := admittedIE.Value.([]ERABAdmitted)
	if !ok || len(admitted) != 1 || admitted[0].ERABID != 5 || admitted[0].GTPTEID != 0x99 {
		t.Fatalf("admitted list = %+v", admittedIE.Value)
	}

	if _, ok := findIE(msg.Value.IEs, idERABFailedToSetupListHOReqAck); !ok {
		t.Fatal("failed-to-setup list IE missing")
	}
}

func TestDecodeMMEStatusTransfer(t *testing.T) {
	b, err := (&lib.MMEStatusTransfer{
		MMEUES1APID: 7,
		ENBUES1APID: 55,
		Container:   lib.StatusTransferContainer{0xde, 0xad, 0xbe, 0xef},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(b)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	containerIE := mustIE(t, msg, idENBStatusTransferContainer)
	if containerIE.Value != "deadbeef" {
		t.Fatalf("container = %v, want deadbeef", containerIE.Value)
	}
}
