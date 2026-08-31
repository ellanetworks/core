// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

const ranStatusContainerHex = "000000000100000000010000"

func validRANStatusContainer(t *testing.T) ngap.StatusTransferContainer {
	t.Helper()

	b, err := hex.DecodeString(ranStatusContainerHex)
	if err != nil {
		t.Fatalf("decode RAN status container vector: %v", err)
	}

	return ngap.StatusTransferContainer(b)
}

// TS 38.413 §8.4.6
func TestUplinkRanStatusTransfer_RelaysToTarget(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	sourceRan := &amf.Radio{Conn: sourceNGAPSender, Log: logger.AmfLog}
	msg := &ngap.UplinkRANStatusTransfer{
		AMFUENGAPID: 100,
		RANUENGAPID: 10,
		Container:   validRANStatusContainer(t),
	}

	HandleUplinkRanStatusTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanStatusTransfers) != 1 {
		t.Fatalf("expected 1 DownlinkRANStatusTransfer relayed to the target, got %d", len(targetSender.SentDownlinkRanStatusTransfers))
	}

	relayed := targetSender.SentDownlinkRanStatusTransfers[0]

	if relayed.AMFUENGAPID != 1 || relayed.RANUENGAPID != 2 {
		t.Fatalf("relayed IDs = amf %d / ran %d, want target 1 / 2", relayed.AMFUENGAPID, relayed.RANUENGAPID)
	}

	if len(relayed.Container) == 0 {
		t.Fatal("expected the transparent container to be relayed")
	}
}

func TestUplinkRanStatusTransfer_NoHandover_Dropped(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)
	targetSender := targetRan.Conn.(*fakeNGAPSender)

	sourceUe := amfInstance.FindUEByAmfUeNgapID(&amf.Radio{Conn: sourceNGAPSender}, 100)
	amfInstance.ClearHandover(sourceUe.UeContext())

	sourceRan := &amf.Radio{Conn: sourceNGAPSender, Log: logger.AmfLog}
	msg := &ngap.UplinkRANStatusTransfer{
		AMFUENGAPID: 100,
		RANUENGAPID: 10,
		Container:   validRANStatusContainer(t),
	}

	HandleUplinkRanStatusTransfer(context.Background(), amfInstance, sourceRan, msg)

	if len(targetSender.SentDownlinkRanStatusTransfers) != 0 {
		t.Fatalf("expected no relay with no handover in progress, got %d", len(targetSender.SentDownlinkRanStatusTransfers))
	}
}
