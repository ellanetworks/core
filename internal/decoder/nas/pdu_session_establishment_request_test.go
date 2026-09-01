// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestExtendedPCOKnownContainers(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   uint16
		want func(*ExtendedProtocolConfigurationOptions) *bool
	}{
		{"ifom support request", 0x000f, func(o *ExtendedProtocolConfigurationOptions) *bool { return o.IFOMSupportRequestUL }},
		{"ipv4 link mtu request", 0x0010, func(o *ExtendedProtocolConfigurationOptions) *bool { return o.IPv4LinkMTURequestUL }},
		{"local address in tft", 0x0011, func(o *ExtendedProtocolConfigurationOptions) *bool {
			return o.MSSupportOfLocalAddressInTFTIndicatorUL
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := extendedPCOFromNAS(nas.ProtocolConfigurationOptions{
				Containers: []nas.PCOContainer{{ID: tc.id}},
			})

			if out.Error != "" {
				t.Fatalf("container 0x%04x reported an error: %s", tc.id, out.Error)
			}

			if got := tc.want(out); got == nil || !*got {
				t.Errorf("container 0x%04x did not set its flag", tc.id)
			}
		})
	}
}

func TestExtendedPCOReportsUnknownContainer(t *testing.T) {
	out := extendedPCOFromNAS(nas.ProtocolConfigurationOptions{
		Containers: []nas.PCOContainer{{ID: 0xabcd}},
	})

	if out.Error == "" {
		t.Fatal("expected an error for an unmodelled container ID")
	}
}
