// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// goldenENBConfigTransfer is an ENB CONFIGURATION TRANSFER initiatingMessage value
// (TS 36.413 §8.15) whose SON Configuration Transfer IE targets eNB 0x00abc
// (PLMN 001/01, TAC 7), followed by opaque SON payload bytes.
const goldenENBConfigTransfer = "000001008140110000f1100000abc000f1100007deadbeef"

func targetENBID() s1ap.GlobalENBID {
	return s1ap.GlobalENBID{
		PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
		ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 0x00abc},
	}
}

func TestHandleENBConfigurationTransfer_RelaysToTarget(t *testing.T) {
	m := newTestMME(t)

	// Target eNB, resolvable by its Global eNB ID.
	targetConn := &captureConn{}
	m.ClaimENBID(mme.NewRadioForTest(targetConn), targetENBID())

	// A distinct source eNB initiates the transfer toward the target.
	sourceConn := &captureConn{}

	value, err := hex.DecodeString(goldenENBConfigTransfer)
	if err != nil {
		t.Fatal(err)
	}

	handleENBConfigurationTransfer(m, context.Background(), mme.NewRadioForTest(sourceConn), value)

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
}

func TestHandleENBConfigurationTransfer_TargetNotConnected(t *testing.T) {
	m := newTestMME(t)
	sourceConn := &captureConn{}

	value, err := hex.DecodeString(goldenENBConfigTransfer)
	if err != nil {
		t.Fatal(err)
	}

	handleENBConfigurationTransfer(m, context.Background(), mme.NewRadioForTest(sourceConn), value)

	if sourceConn.count() != 0 {
		t.Fatalf("no relay expected when the target eNB is not connected, got %d", sourceConn.count())
	}
}
