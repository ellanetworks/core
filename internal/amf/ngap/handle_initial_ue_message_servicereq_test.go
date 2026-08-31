// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	amfnas "github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

type realNASAdapter struct{ amf *amf.AMF }

func (n *realNASAdapter) HandleNAS(ctx context.Context, ue *amf.UeConn, pdu []byte) {
	amfnas.HandleNAS(ctx, n.amf, ue, pdu)
}

// TS 24.501 §5.6.1.8
func TestHandleInitialUEMessage_MalformedServiceRequest_Rejects96(t *testing.T) {
	amfInstance := newTestAMF()
	amfInstance.NAS = &realNASAdapter{amf: amfInstance}

	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleInitialUEMessage(context.Background(), amfInstance, ran, &ngap.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU{0x7e, 0x00, 0x4c},
	})

	if len(sender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("a recognizable service request must never be dropped: want 1 downlink (SERVICE REJECT), got %d", len(sender.SentDownlinkNASTransport))
	}

	pdu := sender.SentDownlinkNASTransport[0].NASPDU
	if len(pdu) < 4 || pdu[2] != uint8(fgs.MsgServiceReject) {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != 0x60 {
		t.Errorf("5GMM cause = 0x%02x, want #96 (invalid mandatory information)", pdu[3])
	}
}
