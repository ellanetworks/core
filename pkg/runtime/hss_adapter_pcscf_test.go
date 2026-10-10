// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

func TestPCSCFReachableNeedsAnAddressInAFamilyOfTheIMSPools(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	adapter := &hssDBAdapter{db: database}

	reachable := func(addresses ...string) bool {
		t.Helper()

		parsed := make([]netip.Addr, 0, len(addresses))
		for _, a := range addresses {
			parsed = append(parsed, netip.MustParseAddr(a))
		}

		if err := database.ReplacePCSCFAddresses(ctx, parsed); err != nil {
			t.Fatalf("ReplacePCSCFAddresses: %s", err)
		}

		ok, err := adapter.PCSCFReachable(ctx)
		if err != nil {
			t.Fatalf("PCSCFReachable: %s", err)
		}

		return ok
	}

	if reachable("10.6.0.5") {
		t.Fatal("reachable without an ims data network")
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	cases := []struct {
		addresses []string
		want      bool
	}{
		{nil, false},
		{[]string{"2001:db8::5"}, false},
		{[]string{"2001:db8::5", "10.6.0.5"}, true},
	}

	for _, tc := range cases {
		if got := reachable(tc.addresses...); got != tc.want {
			t.Errorf("P-CSCF %v on an IPv4-only ims data network: reachable = %t, want %t", tc.addresses, got, tc.want)
		}
	}
}
