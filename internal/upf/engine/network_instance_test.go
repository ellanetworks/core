// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine_test

import (
	"errors"
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
