// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
)

func TestCollectUEPoolsIncludesIPv6(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	dbInstance, err := db.NewDatabaseWithoutRaft(ctx, dbPath)
	if err != nil {
		t.Fatalf("couldn't create database: %s", err)
	}

	defer func() { _ = dbInstance.Close() }()

	if err := dbInstance.CreateDataNetwork(ctx, &db.DataNetwork{
		Name:     "dn-dual",
		IPv4Pool: "10.60.0.0/24",
		IPv6Pool: "2001:db8::/48",
		DNS:      "8.8.8.8",
		MTU:      1500,
	}); err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	pools := CollectUEPools(ctx, dbInstance)

	wantV4 := netip.MustParsePrefix("10.60.0.0/24")
	wantV6 := netip.MustParsePrefix("2001:db8::/48")

	var haveV4, haveV6 bool

	for _, p := range pools {
		switch p {
		case wantV4:
			haveV4 = true
		case wantV6:
			haveV6 = true
		}
	}

	if !haveV4 {
		t.Errorf("expected IPv4 UE pool %s in collected pools, got %v", wantV4, pools)
	}

	if !haveV6 {
		t.Errorf("expected IPv6 UE pool %s in collected pools, got %v", wantV6, pools)
	}

	// The IPv6 pool must reach the BGP safety filter so a peer-advertised
	// route overlapping a UE IPv6 pool is rejected rather than installed.
	filter := bgp.BuildRouteFilter(pools, netip.Addr{}, "")

	var v6InReject bool

	for _, p := range filter.RejectPrefixes {
		if p == wantV6 {
			v6InReject = true
			break
		}
	}

	if !v6InReject {
		t.Errorf("expected IPv6 UE pool %s in BGP reject prefixes, got %v", wantV6, filter.RejectPrefixes)
	}
}
