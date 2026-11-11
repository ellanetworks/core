// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
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

	mcx := &db.DataNetwork{Name: "mcx", IPv4Pool: "10.46.0.0/16", DNS: "8.8.8.8", MTU: 1400, IMS: true}
	if err := database.CreateDataNetwork(ctx, mcx); err != nil {
		t.Fatalf("CreateDataNetwork mcx: %s", err)
	}

	slice, err := database.GetNetworkSlice(ctx, db.InitialSliceName)
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	defaultSlice := models.Snssai{Sst: slice.Sst}

	ims0, err := (&smfDBAdapter{db: database}).ResolveDNN(ctx, "mcx")
	if err != nil {
		t.Fatalf("ResolveDNN mcx: %s", err)
	}

	if _, err := ims0.Config(ctx, defaultSlice); !errors.Is(err, smf.ErrDNNNotInSlice) {
		t.Fatalf("Config of a data network the slice does not serve = %v, want ErrDNNNotInSlice", err)
	}

	if err := database.CreateSliceDataNetwork(ctx, &db.SliceDataNetwork{SliceID: slice.ID, DataNetworkID: mcx.ID, LocalSwitch: true}); err != nil {
		t.Fatalf("CreateSliceDataNetwork: %s", err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, []string{"2001:db8::5", "pcscf.example.org", "10.0.0.5"}); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	resolver := fakeResolver{"pcscf.example.org": {netip.MustParseAddr("10.0.0.7"), netip.MustParseAddr("2001:db8::7")}}
	store := &smfDBAdapter{db: database, resolver: resolver}

	want := []netip.Addr{netip.MustParseAddr("2001:db8::5"), netip.MustParseAddr("10.0.0.7"), netip.MustParseAddr("2001:db8::7"), netip.MustParseAddr("10.0.0.5")}

	ims, err := store.ResolveDNN(ctx, "mcx")
	if err != nil {
		t.Fatalf("ResolveDNN ims: %s", err)
	}

	imsConfig, err := ims.Config(ctx, defaultSlice)
	if err != nil {
		t.Fatalf("Config ims: %s", err)
	}

	if got := imsConfig.PCSCF; !slices.Equal(got, want) || !imsConfig.IMS || imsConfig.NAT || !imsConfig.LocalSwitch {
		t.Fatalf("mcx config = %+v, want IMS with P-CSCF %v", imsConfig, want)
	}

	internet, err := store.ResolveDNN(ctx, db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("ResolveDNN internet: %s", err)
	}

	internetConfig, err := internet.Config(ctx, defaultSlice)
	if err != nil {
		t.Fatalf("Config internet: %s", err)
	}

	if got := internetConfig.PCSCF; len(got) != 0 || internetConfig.IMS || !internetConfig.NAT || internetConfig.LocalSwitch {
		t.Fatalf("internet config = %+v, want no IMS and no P-CSCF", internetConfig)
	}
}

type fakeResolver map[string][]netip.Addr

func (r fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	addrs, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}

	return addrs, nil
}

func TestResolvePCSCFAddresses(t *testing.T) {
	parse := func(s ...string) []netip.Addr {
		out := make([]netip.Addr, 0, len(s))
		for _, a := range s {
			out = append(out, netip.MustParseAddr(a))
		}

		return out
	}

	resolver := fakeResolver{
		"a.example":    parse("10.0.0.1", "10.0.0.2"),
		"b.example":    parse("::ffff:10.0.0.3", "10.0.0.4", "2001:db8::1"),
		"loopback":     parse("127.0.0.1", "::1"),
		"duplicate":    parse("10.0.0.1", "2001:db8::1"),
		"c.example":    parse("10.0.0.6", "10.0.0.7"),
		"ipv6.example": parse("2001:db8::a", "2001:db8::b", "2001:db8::c", "2001:db8::d", "2001:db8::e"),
	}

	cases := []struct {
		name    string
		entries []string
		want    []netip.Addr
	}{
		{"none", nil, nil},
		{"addresses only", []string{"10.0.0.5", "2001:db8::5"}, parse("10.0.0.5", "2001:db8::5")},
		{"host names keep their place in the order", []string{"2001:db8::5", "a.example", "10.0.0.5"}, parse("2001:db8::5", "10.0.0.1", "10.0.0.2", "10.0.0.5")},
		{"a host name that does not resolve is left out", []string{"missing.example", "10.0.0.5"}, parse("10.0.0.5")},
		{"no host name resolves", []string{"missing.example"}, nil},
		{"at most five addresses of each family", []string{"a.example", "b.example", "c.example", "ipv6.example"}, parse("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "2001:db8::1", "10.0.0.6", "2001:db8::a", "2001:db8::b", "2001:db8::c", "2001:db8::d")},
		{"addresses that are not unicast are left out", []string{"loopback", "10.0.0.5"}, parse("10.0.0.5")},
		{"duplicates are left out", []string{"a.example", "duplicate"}, parse("10.0.0.1", "10.0.0.2", "2001:db8::1")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolvePCSCFAddresses(context.Background(), resolver, tc.entries); !slices.Equal(got, tc.want) {
				t.Fatalf("resolvePCSCFAddresses(%v) = %v, want %v", tc.entries, got, tc.want)
			}
		})
	}
}
