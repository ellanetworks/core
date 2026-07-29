// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type PduSessionEstablishmentRequestOpts struct {
	PDUSessionID   uint8
	PDUSessionType fgs.PDUSessionType
}

func BuildPduSessionEstablishmentRequest(opts *PduSessionEstablishmentRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PduSessionEstablishmentRequestOpts is nil")
	}

	pduSessionType := opts.PDUSessionType

	extendedPCO := uePDUEstablishmentPCO()

	m := &fgs.PDUSessionEstablishmentRequest{
		PDUSessionID:             fgs.PDUSessionID(opts.PDUSessionID),
		PTI:                      0x01,
		IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
		PDUSessionType:           &pduSessionType,
		ExtendedPCO:              &extendedPCO,
	}

	return m.MarshalBinary()
}

// uePDUEstablishmentPCO builds the PCO the UE requests at PDU session
// establishment (TS 24.008 §10.5.6.3): IP address allocation via NAS signalling,
// plus DNS server IPv4 and IPv6 address requests, each an empty-content container.
func uePDUEstablishmentPCO() nas.ProtocolConfigurationOptions {
	return nas.NewRequestedProtocolConfigurationOptions(
		nas.PCOContainerIPAddressAllocationViaNAS,
		nas.PCOContainerDNSServerIPv4Address,
		nas.PCOContainerDNSServerIPv6Address,
	)
}
