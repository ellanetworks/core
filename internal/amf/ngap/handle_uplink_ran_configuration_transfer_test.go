// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// goldenUplinkRANConfigTransfer is an UPLINK RAN CONFIGURATION TRANSFER whose
// SON Configuration Transfer targets gNB 00:01:02 in PLMN 001/01.
const goldenUplinkRANConfigTransfer = "0030402700000100634020000000f110100001020000f1100000010000f11010000a0b0000f11000000200"

// uplinkTransferFixture returns the parsed message and the SON payload it
// carries, which is what the AMF must relay untouched.
func uplinkTransferFixture(t *testing.T) *ngap.UplinkRANConfigurationTransfer {
	t.Helper()

	raw, err := hex.DecodeString(goldenUplinkRANConfigTransfer)
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := ngap.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		t.Fatalf("got %T, want an initiating message", pdu)
	}

	msg, err := ngap.ParseUplinkRANConfigurationTransfer(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return msg
}

func TestUplinkRANConfigurationTransfer_NilSONConfiguration(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	// An absent IE is a well-formed message; the relay simply has nothing to do.
	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, ran,
		&ngap.UplinkRANConfigurationTransfer{})
}

func TestUplinkRANConfigurationTransfer_TargetRanNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, ran, uplinkTransferFixture(t))
}

// A transfer whose leading Target RAN Node ID does not decode must be dropped
// rather than routed on a zero target.
func TestUplinkRANConfigurationTransfer_UndecodableTargetIsDropped(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, ran,
		&ngap.UplinkRANConfigurationTransfer{SONConfigurationTransfer: ngap.SONConfigurationTransfer{0x00}})
}

// TS 38.413 §8.8.1.2: the AMF "shall transparently transfer the SON
// Configuration Transfer IE towards the NG-RAN node indicated in the Target RAN
// Node ID IE". The fake sender decodes what was written with the reference
// implementation, so this also checks a third party can read what we emit.
func TestUplinkRANConfigurationTransfer_ForwardsToTargetRan(t *testing.T) {
	sourceRan := newTestRadio(newTestAMF())
	msg := uplinkTransferFixture(t)

	target, err := msg.SONConfigurationTransfer.TargetRANNodeID()
	if err != nil {
		t.Fatalf("TargetRANNodeID: %v", err)
	}

	// Register the target the fixture actually names, so the test cannot drift
	// from the golden vector.
	targetID := util.RANNodeIDToModels(target.GlobalRANNodeID)
	targetSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		RanPresent: amf.RanPresentGNbID,
		RanID:      &targetID,
		Conn:       targetSender,
		Log:        sourceRan.Log,
	}

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanConfigTransfers) != 1 {
		t.Fatalf("expected 1 Downlink RAN Configuration Transfer, got %d", len(targetSender.SentDownlinkRanConfigTransfers))
	}

	got, err := targetSender.SentDownlinkRanConfigTransfers[0].SONConfigurationTransfer.TargetRANNodeID()
	if err != nil {
		t.Fatalf("forwarded transfer does not decode: %v", err)
	}

	if got.GlobalRANNodeID.Kind != ngap.RANNodeIDGNB {
		t.Fatalf("forwarded transfer lost its target gNB ID: kind = %d", got.GlobalRANNodeID.Kind)
	}

	if want := "000102"; got.GlobalRANNodeID.Hex() != want {
		t.Errorf("forwarded target gNB ID = %s, want %s", got.GlobalRANNodeID.Hex(), want)
	}
}
