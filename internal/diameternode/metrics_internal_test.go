// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestConnectedPeersCountsOpenPeersByRole(t *testing.T) {
	smsc, ims := connectedPeers([]PeerStatus{
		{ID: "smsc-1", Role: PeerRoleSMSC, State: diameter.PeerOpen},
		{ID: "smsc-2", Role: PeerRoleSMSC, State: diameter.PeerDown},
		{Role: PeerRoleIMS, Host: "scscf.ims.example.org", State: diameter.PeerOpen},
		{Role: PeerRoleIMS, Host: "pcscf.ims.example.org", State: diameter.PeerOpen},
		{Role: PeerRoleIMS, Host: "old.ims.example.org", State: diameter.PeerClosing},
	})

	if smsc != 1 || ims != 2 {
		t.Fatalf("connected = %d SMSCs and %d IMS nodes, want 1 and 2", smsc, ims)
	}
}
