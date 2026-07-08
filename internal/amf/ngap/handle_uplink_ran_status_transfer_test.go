// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// validRANStatusContainer builds a re-encodable transparent container carrying one DRB's
// PDCP SN/HFN status (the list has a lower bound of 1).
func validRANStatusContainer() *ngapType.RANStatusTransferTransparentContainer {
	return &ngapType.RANStatusTransferTransparentContainer{
		DRBsSubjectToStatusTransferList: ngapType.DRBsSubjectToStatusTransferList{
			List: []ngapType.DRBsSubjectToStatusTransferItem{
				{
					DRBID: ngapType.DRBID{Value: 1},
					DRBStatusUL: ngapType.DRBStatusUL{
						Present:       ngapType.DRBStatusULPresentDRBStatusUL12,
						DRBStatusUL12: &ngapType.DRBStatusUL12{ULCOUNTValue: ngapType.COUNTValueForPDCPSN12{PDCPSN12: 1}},
					},
					DRBStatusDL: ngapType.DRBStatusDL{
						Present:       ngapType.DRBStatusDLPresentDRBStatusDL12,
						DRBStatusDL12: &ngapType.DRBStatusDL12{DLCOUNTValue: ngapType.COUNTValueForPDCPSN12{PDCPSN12: 1}},
					},
				},
			},
		},
	}
}

// A UPLINK RAN STATUS TRANSFER arriving on the source during an in-progress N2 handover
// is relayed to the target as a DOWNLINK RAN STATUS TRANSFER re-stamped with the
// target's UE IDs, carrying the transparent container verbatim (TS 38.413 §8.4.6/7).
func TestUplinkRanStatusTransfer_RelaysToTarget(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	// The transfer arrives on the source association, carrying the source UE's IDs.
	sourceRan := &amf.Radio{Conn: sourceNGAPSender, Log: logger.AmfLog}
	msg := decode.UplinkRANStatusTransfer{
		AMFUENGAPID: 100,
		RANUENGAPID: 10,
		Container:   validRANStatusContainer(),
	}

	ngap.HandleUplinkRanStatusTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanStatusTransfers) != 1 {
		t.Fatalf("expected 1 DownlinkRANStatusTransfer relayed to the target, got %d", len(targetSender.SentDownlinkRanStatusTransfers))
	}

	var (
		amfID, ranID  int64
		haveContainer bool
	)

	for _, ie := range targetSender.SentDownlinkRanStatusTransfers[0].ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfID = ie.Value.AMFUENGAPID.Value
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranID = ie.Value.RANUENGAPID.Value
		case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
			haveContainer = ie.Value.RANStatusTransferTransparentContainer != nil
		}
	}

	// Re-stamped with the target UE's IDs (target = NewUeConnForTest(ran, ran=2, amf=1)).
	if amfID != 1 || ranID != 2 {
		t.Fatalf("relayed IDs = amf %d / ran %d, want target 1 / 2", amfID, ranID)
	}

	if !haveContainer {
		t.Fatal("expected the transparent container to be relayed")
	}
}

// With no handover in progress there is no target, so the transfer is dropped.
func TestUplinkRanStatusTransfer_NoHandover_Dropped(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	sourceUe := amfInstance.FindUEByAmfUeNgapID(&amf.Radio{Conn: sourceNGAPSender}, 100)
	amfInstance.ClearHandover(sourceUe.UeContext())

	sourceRan := &amf.Radio{Conn: sourceNGAPSender, Log: logger.AmfLog}
	msg := decode.UplinkRANStatusTransfer{
		AMFUENGAPID: 100,
		RANUENGAPID: 10,
		Container:   &ngapType.RANStatusTransferTransparentContainer{},
	}

	ngap.HandleUplinkRanStatusTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanStatusTransfers) != 0 {
		t.Fatalf("expected no relay with no handover in progress, got %d", len(targetSender.SentDownlinkRanStatusTransfers))
	}
}
