// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestNetworkInstanceIDs(t *testing.T) {
	conn := &engine.SessionEngine{}
	conn.SetNetworkInstances(map[string]uint8{"ims": 1, "corp": 2})

	for _, tc := range []struct {
		name string
		want uint8
	}{
		{name: "", want: 0},
		{name: models.DefaultNetworkInstanceName, want: 0},
		{name: "ims", want: 1},
		{name: "corp", want: 2},
	} {
		got, err := conn.NetworkInstanceIDForTest(tc.name)
		if err != nil || got != tc.want {
			t.Fatalf("network instance %q = %d, %v; want %d", tc.name, got, err, tc.want)
		}
	}

	if _, err := conn.NetworkInstanceIDForTest("mc"); !errors.Is(err, models.ErrNetworkInstanceUnavailable) {
		t.Fatalf("unknown network instance = %v, want ErrNetworkInstanceUnavailable", err)
	}

	if !conn.HasNetworkInstance("ims") || conn.HasNetworkInstance("mc") {
		t.Fatal("HasNetworkInstance disagrees with the configured instances")
	}
}

func TestDataNetworkTagsAreReleased(t *testing.T) {
	conn := &engine.SessionEngine{}

	acquire := func(id string) uint16 {
		t.Helper()

		tag, err := conn.AcquireDataNetworkTagForTest(id)
		if err != nil {
			t.Fatalf("acquire %q: %v", id, err)
		}

		return tag
	}

	if tag := acquire(""); tag != 0 {
		t.Fatalf("tag of no data network = %d, want 0", tag)
	}

	a := acquire("1/internet")
	if again := acquire("1/internet"); again != a {
		t.Fatalf("second session of a pair got tag %d, want %d", again, a)
	}

	b := acquire("1/ims")
	if b == a {
		t.Fatalf("two pairs share tag %d", a)
	}

	conn.ReleaseDataNetworkTagForTest("1/internet")

	c := acquire("2/internet")
	if c == a || c == b {
		t.Fatalf("tag %d reused while a session of its pair is live", c)
	}

	conn.ReleaseDataNetworkTagForTest("1/internet")
	conn.ReleaseDataNetworkTagForTest("1/internet")

	if again := acquire("1/internet"); again == b || again == c {
		t.Fatalf("released pair got tag %d, which a live pair holds", again)
	}
}

func TestModifyToAnUnavailableNetworkInstanceChangesNothing(t *testing.T) {
	rm, err := engine.NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, nil, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	conn.SetNetworkInstances(map[string]uint8{"ims": 1})

	const seid = uint64(41)

	if _, err := conn.EstablishSession(context.Background(), &models.EstablishRequest{SEID: seid, IMSI: "001010000000001", NetworkInstance: "ims"}); err != nil {
		t.Fatalf("establish: %v", err)
	}

	_, err = conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID:       seid,
		Forwarding: &models.SessionForwarding{NATExempt: true, NetworkInstance: "mc"},
	})
	if !errors.Is(err, models.ErrNetworkInstanceUnavailable) {
		t.Fatalf("modify to an unknown network instance = %v, want ErrNetworkInstanceUnavailable", err)
	}

	if id := conn.GetSession(seid).NetworkInstanceForTest(); id != 1 {
		t.Fatalf("network instance after a refused modify = %d, want 1", id)
	}

	if _, err := conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID:       seid,
		Forwarding: &models.SessionForwarding{NetworkInstance: models.DefaultNetworkInstanceName},
	}); err != nil {
		t.Fatalf("modify to the default network instance: %v", err)
	}

	if id := conn.GetSession(seid).NetworkInstanceForTest(); id != 0 {
		t.Fatalf("network instance after moving to default = %d, want 0", id)
	}
}
