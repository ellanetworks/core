// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"net/netip"

	libngap "github.com/ellanetworks/core/ngap"
)

func BuildPathSwitchRequestAcknowledgeTransfer(teid uint32, n3IPv4 netip.Addr, n3IPv6 netip.Addr) ([]byte, error) {
	tla, err := encodeTransportLayerAddress(n3IPv4, n3IPv6)
	if err != nil {
		return nil, fmt.Errorf("encode transport layer address failed: %s", err)
	}

	transfer := libngap.PathSwitchRequestAcknowledgeTransfer{
		ULNGUUPTNLInformation: &libngap.UPTransportLayerInformation{
			GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: tla, GTPTEID: libngap.GTPTEID(teid)},
		},
		SecurityIndication: &libngap.SecurityIndication{
			IntegrityProtectionIndication:       libngap.IntegrityProtectionNotNeeded,
			ConfidentialityProtectionIndication: libngap.ConfidentialityProtectionNotNeeded,
		},
	}

	buf, err := transfer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode path switch request acknowledge transfer: %s", err)
	}

	return buf, nil
}
