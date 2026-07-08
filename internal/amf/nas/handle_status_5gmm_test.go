// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestHandleStatus5GMM_UEDeregistered_Ignored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Deregister(context.TODO())

	m := buildTestStatus5gmm()

	handleStatus5GMM(context.Background(), ue, m.Status5GMM)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected Status 5GMM in Deregistered state to be ignored, but a downlink was sent")
	}
}

func TestHandleStatus5GMM_Registered_Ignored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	m := buildTestStatus5gmm()

	handleStatus5GMM(context.Background(), ue, m.Status5GMM)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink for Status 5GMM, but a downlink was sent")
	}
}

func buildTestStatus5gmm() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	status5gmm := nasMessage.NewStatus5GMM(0)
	status5gmm.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	status5gmm.SetSpareHalfOctet(0x00)
	status5gmm.SetMessageType(nas.MsgTypeStatus5GMM)

	m.Status5GMM = status5gmm
	m.SetMessageType(nas.MsgTypeStatus5GMM)

	return m
}
