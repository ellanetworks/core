// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestEmmMessageTypeNameGenericNASTransport(t *testing.T) {
	if got := EmmMessageTypeName(eps.MsgDownlinkGenericNASTransport); got != "DownlinkGenericNASTransport" {
		t.Errorf("0x68 = %q", got)
	}

	if got := EmmMessageTypeName(eps.MsgUplinkGenericNASTransport); got != "UplinkGenericNASTransport" {
		t.Errorf("0x69 = %q", got)
	}
}
