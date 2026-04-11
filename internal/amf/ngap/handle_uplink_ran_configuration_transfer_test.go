// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func TestUplinkRanConfigurationTransfer_NilSONConfiguration(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amfInstance, ran, decode.UplinkRANConfigurationTransfer{})
}

func TestUplinkRanConfigurationTransfer_TargetRanNotFound(t *testing.T) {
	ran := newTestRadio()
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
	sourceRan := newTestRadio()

	targetSender := &FakeNGAPSender{}
	targetRan := &amf.Radio{
		RanPresent: amf.RanPresentGNbID,
		RanID: &models.GlobalRanNodeID{
			GNbID: &models.GNbID{
				GNBValue:  "000001",
				BitLength: 22,
			},
		},
		NGAPSender: targetSender,
		RanUEs:     make(map[int64]*amf.RanUe),
		Log:        sourceRan.Log,
	}

	amfInstance := newTestAMFWithSmf(&FakeSmfSbi{})
	amfInstance.Radios[new(sctp.SCTPConn)] = targetRan

	sonTransfer := &ngapType.SONConfigurationTransfer{
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
	}

	msg := decode.UplinkRANConfigurationTransfer{
		SONConfigurationTransferUL: sonTransfer,
	}

	ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanConfigTransfers) != 1 {
		t.Fatalf("expected 1 SendDownlinkRanConfigurationTransfer call, got %d", len(targetSender.SentDownlinkRanConfigTransfers))
	}

	if targetSender.SentDownlinkRanConfigTransfers[0] != sonTransfer {
		t.Error("expected forwarded SON transfer to match input")
	}
}
