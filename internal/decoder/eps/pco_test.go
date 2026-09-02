// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestExtendedPCODecodesIPv4LinkMTURequest(t *testing.T) {
	out := ExtendedPCO(&nas.ProtocolConfigurationOptions{
		Containers: []nas.PCOContainer{{ID: 0x0010}},
	})

	if out.Error != "" {
		t.Fatalf("container 0x0010 reported an error: %s", out.Error)
	}

	if out.IPv4LinkMTURequestUL == nil || !*out.IPv4LinkMTURequestUL {
		t.Error("container 0x0010 did not set ipv4_link_mtu_request_ul")
	}
}
