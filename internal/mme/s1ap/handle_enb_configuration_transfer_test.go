// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func targetENBID() s1ap.GlobalENBID {
	return s1ap.GlobalENBID{
		PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
		ENBID:        s1ap.ENBID{MacroENBID: s1ap.Ptr(s1ap.MacroENBID(0x00abc))},
	}
}

func enbConfigTransferValue(t *testing.T) []byte {
	t.Helper()

	msg := &s1ap.ENBConfigurationTransfer{SONConfigurationTransfer: &s1ap.SONConfigurationTransfer{
		TargeteNBID: s1ap.TargeteNBID{
			GlobalENBID: targetENBID(),
			SelectedTAI: s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7},
		},
		SourceeNBID: s1ap.SourceeNBID{
			GlobalENBID: s1ap.GlobalENBID{
				PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
				ENBID:        s1ap.ENBID{MacroENBID: s1ap.Ptr(s1ap.MacroENBID(0x00abd))},
			},
			SelectedTAI: s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 8},
		},
		SONInformation: s1ap.SONInformation{SONInformationRequest: s1ap.Ptr(s1ap.SONInformationRequestTimeSynchronisationInfo)},
	}}

	return initiatingValue(t, mustMarshal(t, msg.Marshal))
}

func TestHandleENBConfigurationTransfer_RelaysToTarget(t *testing.T) {
	m := newTestMME(t)

	targetConn := &captureConn{}
	if err := m.ClaimENBID(t.Context(), mme.NewRadioForTest(targetConn), targetENBID(), mme.DefaultRelativeCapacity); err != nil {
		t.Fatalf("ClaimENBID: %v", err)
	}

	sourceConn := &captureConn{}

	handleENBConfigurationTransfer(context.Background(), m, mme.NewRadioForTest(sourceConn), enbConfigTransferValue(t))

	if targetConn.count() != 1 {
		t.Fatalf("expected 1 relayed message to the target eNB, got %d", targetConn.count())
	}

	if sourceConn.count() != 0 {
		t.Fatalf("source eNB must not receive the relay, got %d", sourceConn.count())
	}

	pdu, err := s1ap.Unmarshal(targetConn.sent[0])
	if err != nil {
		t.Fatalf("Unmarshal relayed PDU: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcMMEConfigurationTransfer {
		t.Fatalf("expected MME CONFIGURATION TRANSFER (proc %d), got %T", s1ap.ProcMMEConfigurationTransfer, pdu)
	}

	if targetConn.streams[0] != mme.S1apStreamNonUE {
		t.Fatalf("MME CONFIGURATION TRANSFER sent on stream %d, want the non-UE-associated stream %d (TS 36.413 §8.16.1)",
			targetConn.streams[0], mme.S1apStreamNonUE)
	}
}

func TestHandleENBConfigurationTransfer_TargetNotConnected(t *testing.T) {
	m := newTestMME(t)
	sourceConn := &captureConn{}

	handleENBConfigurationTransfer(context.Background(), m, mme.NewRadioForTest(sourceConn), enbConfigTransferValue(t))

	if sourceConn.count() != 0 {
		t.Fatalf("no relay expected when the target eNB is not connected, got %d", sourceConn.count())
	}
}
