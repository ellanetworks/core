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

const goldenUplinkRANConfigTransfer = "0030402700000100634020000000f110100001020000f1100000010000f11010000a0b0000f11000000200"

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
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, ran,
		&ngap.UplinkRANConfigurationTransfer{})

	assertNoRANConfigurationTransferRelayed(t, sender)
}

func TestUplinkRANConfigurationTransfer_TargetRanNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)
	amfInstance := newTestAMF()

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, ran, uplinkTransferFixture(t))

	assertNoRANConfigurationTransferRelayed(t, sender)
}

func TestUplinkRANConfigurationTransfer_UndecodableTargetIsDropped(t *testing.T) {
	sourceRan := newTestRadio(newTestAMF())
	sourceSender := sourceRan.Conn.(*fakeNGAPSender)

	target, err := uplinkTransferFixture(t).SONConfigurationTransfer.TargetRANNodeID()
	if err != nil {
		t.Fatalf("TargetRANNodeID: %v", err)
	}

	targetID := util.RANNodeIDToModels(target.GlobalRANNodeID)
	targetSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		RanPresent: amf.RanPresentGNbID,
		RanID:      &targetID,
		Conn:       targetSender,
		Log:        sourceRan.Log,
	}

	amfInstance := newTestAMF()
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	HandleUplinkRANConfigurationTransfer(context.Background(), amfInstance, sourceRan,
		&ngap.UplinkRANConfigurationTransfer{SONConfigurationTransfer: ngap.SONConfigurationTransfer{0x00}})

	assertNoRANConfigurationTransferRelayed(t, sourceSender)
	assertNoRANConfigurationTransferRelayed(t, targetSender)
}

func assertNoRANConfigurationTransferRelayed(t *testing.T, sender *fakeNGAPSender) {
	t.Helper()

	if len(sender.SentDownlinkRanConfigTransfers) != 0 {
		t.Fatalf("expected the transfer to be dropped, got %d relayed", len(sender.SentDownlinkRanConfigTransfers))
	}
}

// TS 38.413 §8.8.1.2
func TestUplinkRANConfigurationTransfer_ForwardsToTargetRan(t *testing.T) {
	sourceRan := newTestRadio(newTestAMF())
	msg := uplinkTransferFixture(t)

	target, err := msg.SONConfigurationTransfer.TargetRANNodeID()
	if err != nil {
		t.Fatalf("TargetRANNodeID: %v", err)
	}

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
