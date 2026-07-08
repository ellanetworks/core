// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	amfnas "github.com/ellanetworks/core/internal/amf/nas"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	gonas "github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// realNASAdapter wires the actual NAS layer (not the fake) into the AMF, so a routing test
// exercises the real IsServiceRequest peek and HandleServiceRequest reject end to end.
type realNASAdapter struct{ amf *amf.AMF }

func (n *realNASAdapter) HandleNAS(ctx context.Context, ue *amf.UeConn, pdu []byte) {
	amfnas.HandleNAS(ctx, n.amf, ue, pdu)
}

func (n *realNASAdapter) IsServiceRequest(pdu []byte) bool { return amfnas.IsServiceRequest(pdu) }

func (n *realNASAdapter) HandleServiceRequest(ctx context.Context, ue *amf.UeConn, pdu []byte) {
	amfnas.HandleServiceRequest(ctx, n.amf, ue, pdu)
}

// A truncated-but-recognizable plain SERVICE REQUEST arriving in an Initial UE Message (the
// 3gpp-server Test5GServiceRequest_Fuzz "7e004c" case, sent verbatim) must be classified by
// message type, routed to the dedicated handler, and answered with SERVICE REJECT #96 over
// the whole NGAP→NAS path — never dropped in HandleNAS (TS 24.501 §5.6.1.8 b).
func TestHandleInitialUEMessage_MalformedServiceRequest_Rejects96(t *testing.T) {
	amfInstance := newTestAMF()
	amfInstance.NAS = &realNASAdapter{amf: amfInstance}

	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7e, 0x00, 0x4c}, // plain SERVICE REQUEST header, truncated
	})

	if len(sender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("a recognizable service request must never be dropped: want 1 downlink (SERVICE REJECT), got %d", len(sender.SentDownlinkNASTransport))
	}

	pdu := sender.SentDownlinkNASTransport[0].NasPdu
	if len(pdu) < 4 || pdu[2] != gonas.MsgTypeServiceReject {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != nasMessage.Cause5GMMInvalidMandatoryInformation {
		t.Errorf("5GMM cause = 0x%02x, want #96 (invalid mandatory information)", pdu[3])
	}
}
