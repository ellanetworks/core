// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDataNetworkConfigCarriesPCSCFAddressesOnlyOnIMS(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "mcx", IPv4Pool: "10.46.0.0/16", DNS: "8.8.8.8", MTU: 1400, IMS: true}); err != nil {
		t.Fatalf("CreateDataNetwork mcx: %s", err)
	}

	want := []netip.Addr{netip.MustParseAddr("2001:db8::5"), netip.MustParseAddr("10.0.0.5")}
	if err := database.ReplacePCSCFAddresses(ctx, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	store := &smfDBAdapter{db: database}

	ims, err := store.ResolveDNN(ctx, "mcx")
	if err != nil {
		t.Fatalf("ResolveDNN ims: %s", err)
	}

	imsConfig, err := ims.Config(ctx)
	if err != nil {
		t.Fatalf("Config ims: %s", err)
	}

	if got := imsConfig.PCSCF; !slices.Equal(got, want) || !imsConfig.IMS {
		t.Fatalf("mcx config = %+v, want IMS with P-CSCF %v", imsConfig, want)
	}

	internet, err := store.ResolveDNN(ctx, db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("ResolveDNN internet: %s", err)
	}

	internetConfig, err := internet.Config(ctx)
	if err != nil {
		t.Fatalf("Config internet: %s", err)
	}

	if got := internetConfig.PCSCF; len(got) != 0 || internetConfig.IMS {
		t.Fatalf("internet config = %+v, want no IMS and no P-CSCF", internetConfig)
	}
}
