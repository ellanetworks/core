// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §6.6.1.1 puts the configuration options of a transferred PDN
// connection in the extended element, but the UE reads it only where the network
// advertised support, which it does only for a UE that advertised it first
// (§5.5.1.2.4). Both conditions are needed, so neither one alone selects it.
func TestUsesExtendedPCONeedsBothTheTransferAndTheUECapability(t *testing.T) {
	const (
		advertised    = 0x80
		notAdvertised = 0x00
	)

	for _, tc := range []struct {
		name        string
		requestType eps.RequestType
		octet8      byte
		want        bool
	}{
		{"transferred, UE advertised ePCO", eps.RequestTypeHandover, advertised, true},
		{"transferred, UE did not advertise ePCO", eps.RequestTypeHandover, notAdvertised, false},
		{"initial request, UE advertised ePCO", eps.RequestTypeInitialRequest, advertised, false},
		{"initial request, UE did not advertise ePCO", eps.RequestTypeInitialRequest, notAdvertised, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)

			ue := m.NewUe(&captureConn{}, 7)
			ue.SetUESecurityCapability(eps.UENetworkCapability{
				EEA:  0xf0,
				EIA:  0x70,
				Rest: []byte{0x00, tc.octet8},
			}, nil, MintAuthProofForAttachRequest())

			if got := ue.UsesExtendedPCO(tc.requestType); got != tc.want {
				t.Errorf("UsesExtendedPCO(%v) = %v, want %v", tc.requestType, got, tc.want)
			}
		})
	}
}
