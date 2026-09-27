// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestDecodeDownlinkGenericNASTransportLPP(t *testing.T) {
	msg := decodeHex(t, "0768010006d002000021c0650101")

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Value != int64(eps.MsgDownlinkGenericNASTransport) {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	dl := msg.EMMMessage.DownlinkGenericNASTransport
	if dl == nil {
		t.Fatal("DownlinkGenericNASTransport not decoded")
	}

	if dl.GenericMessageContainerType.Value != int64(eps.GenericMessageContainerTypeLPP) || dl.GenericMessageContainerType.Unknown {
		t.Errorf("container type = %+v", dl.GenericMessageContainerType)
	}

	if dl.GenericMessageContainer.RawHex != "d002000021c0" {
		t.Errorf("container raw = %q", dl.GenericMessageContainer.RawHex)
	}

	if dl.GenericMessageContainer.LppMessage == nil || dl.GenericMessageContainer.LppMessage.Decoded == nil {
		t.Errorf("LPP message not decoded: %+v", dl.GenericMessageContainer.LppMessage)
	}

	if dl.AdditionalInformation == nil {
		t.Error("additional information dropped")
	}
}

func TestDecodeUplinkGenericNASTransportUnknownContainer(t *testing.T) {
	msg := decodeHex(t, "0769020001aa")

	ul := msg.EMMMessage.UplinkGenericNASTransport
	if ul == nil {
		t.Fatal("UplinkGenericNASTransport not decoded")
	}

	if ul.GenericMessageContainer.LppMessage != nil || ul.GenericMessageContainer.Error == "" {
		t.Errorf("container = %+v, want undecoded with an error", ul.GenericMessageContainer)
	}
}
