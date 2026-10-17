// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
)

func TestRxOwnersFindTheNodeHoldingTheAddress(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	dn, err := database.GetDataNetwork(ctx, models.IMSDataNetworkName)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	if err := database.CreateProfile(ctx, &db.Profile{Name: "voice", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	profile, err := database.GetProfile(ctx, "voice")
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	const imsi = "001010000000001"

	subscriber := &db.Subscriber{
		Imsi: imsi, SequenceNumber: "000000000001", PermanentKey: "6f30087629feb0b089783c81d0ae09b5", Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID,
	}

	if err := database.CreateSubscriber(ctx, subscriber); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	if err := database.UpsertClusterMember(ctx, &db.ClusterMember{NodeID: "node-b"}); err != nil {
		t.Fatalf("UpsertClusterMember: %s", err)
	}

	leases := map[string]string{"10.60.0.1": database.RaftID(), "10.60.0.2": "node-b", "10.60.0.3": "node-gone"}

	for i, address := range []string{"10.60.0.1", "10.60.0.2", "10.60.0.3"} {
		session := i + 1
		lease := &db.IPLease{PoolID: dn.ID, PoolType: "ipv4", IMSI: imsi, SessionID: &session, Type: "dynamic", CreatedAt: time.Now().Unix(), NodeID: leases[address]}

		if err := database.CreateLease(ctx, lease, netip.MustParseAddr(address)); err != nil {
			t.Fatalf("CreateLease %s: %s", address, err)
		}
	}

	owners := &rxOwners{db: database, directory: &diameterDirectory{db: database}, port: 3868}

	local, err := owners.Owner(ctx, netip.MustParseAddr("10.60.0.1"))
	if err != nil || !local.Local {
		t.Fatalf("owner of a local address = %+v, %v", local, err)
	}

	remote, err := owners.Owner(ctx, netip.MustParseAddr("10.60.0.2"))
	if err != nil || remote.Local || remote.URI.Port != 3868 || remote.URI.Transport != diameter.TransportSCTP ||
		!strings.HasSuffix(remote.URI.Host, ".mme.epc.mnc001.mcc001.3gppnetwork.org") {
		t.Fatalf("owner on another node = %+v, %v", remote, err)
	}

	for _, address := range []string{"10.60.0.3", "10.60.0.4"} {
		if _, err := owners.Owner(ctx, netip.MustParseAddr(address)); !errors.Is(err, pcf.ErrNoOwner) {
			t.Fatalf("owner of %s: %v, want ErrNoOwner", address, err)
		}
	}
}
