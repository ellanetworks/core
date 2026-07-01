// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func TestUplinkRanConfigurationTransfer_NilSONConfiguration(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amfInstance, ran, decode.UplinkRANConfigurationTransfer{})
}

func TestUplinkRanConfigurationTransfer_TargetRanNotFound(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	msg := decode.UplinkRANConfigurationTransfer{
		SONConfigurationTransferUL: &ngapType.SONConfigurationTransfer{
			TargetRANNodeID: ngapType.TargetRANNodeID{
				GlobalRANNodeID: ngapType.GlobalRANNodeID{
					Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
					GlobalGNBID: &ngapType.GlobalGNBID{
						PLMNIdentity: ngapType.PLMNIdentity{Value: []byte{0x00, 0xF1, 0x10}},
						GNBID: ngapType.GNBID{
							Present: ngapType.GNBIDPresentGNBID,
							GNBID: &aper.BitString{
								Bytes:     []byte{0x00, 0x00, 0x01},
								BitLength: 22,
							},
						},
					},
				},
			},
		},
	}

	ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amfInstance, ran, msg)
}

func TestUplinkRanConfigurationTransfer_ForwardsToTargetRan(t *testing.T) {
	sourceRan := newTestRadio(newTestAMF())

	targetSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{
		RanPresent: amf.RanPresentGNbID,
		RanID: &models.GlobalRanNodeID{
			GNbID: &models.GNbID{
				GNBValue:  "000001",
				BitLength: 22,
			},
		},
		Conn: targetSender,
		Log:  sourceRan.Log,
	}

	amfInstance := newTestAMFWithSmf(&fakeSmfSbi{})
	amfInstance.IndexRadioForTest(new(sctp.SCTPConn), targetRan)

	plmn := []byte{0x00, 0xF1, 0x10}
	gnbID := []byte{0x00, 0x00, 0x01}

	ranNode := func() ngapType.GlobalRANNodeID {
		return ngapType.GlobalRANNodeID{
			Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
			GlobalGNBID: &ngapType.GlobalGNBID{
				PLMNIdentity: ngapType.PLMNIdentity{Value: plmn},
				GNBID: ngapType.GNBID{
					Present: ngapType.GNBIDPresentGNBID,
					GNBID:   &aper.BitString{Bytes: gnbID, BitLength: 22},
				},
			},
		}
	}
	tai := ngapType.TAI{PLMNIdentity: ngapType.PLMNIdentity{Value: plmn}, TAC: ngapType.TAC{Value: []byte{0x00, 0x00, 0x01}}}

	sonTransfer := &ngapType.SONConfigurationTransfer{
		TargetRANNodeID: ngapType.TargetRANNodeID{GlobalRANNodeID: ranNode(), SelectedTAI: tai},
		SourceRANNodeID: ngapType.SourceRANNodeID{GlobalRANNodeID: ranNode(), SelectedTAI: tai},
		SONInformation: ngapType.SONInformation{
			Present:               ngapType.SONInformationPresentSONInformationRequest,
			SONInformationRequest: &ngapType.SONInformationRequest{Value: 0},
		},
	}

	msg := decode.UplinkRANConfigurationTransfer{
		SONConfigurationTransferUL: sonTransfer,
	}

	ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanConfigTransfers) != 1 {
		t.Fatalf("expected 1 SendDownlinkRanConfigurationTransfer call, got %d", len(targetSender.SentDownlinkRanConfigTransfers))
	}

	// The transfer is re-encoded onto the wire, so compare its content, not the
	// pointer: the target gNB ID the AMF forwards must match the input.
	got := targetSender.SentDownlinkRanConfigTransfers[0]
	if got.TargetRANNodeID.GlobalRANNodeID.GlobalGNBID == nil ||
		!bytes.Equal(got.TargetRANNodeID.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID.Bytes, gnbID) {
		t.Error("expected forwarded SON transfer to match input")
	}
}
