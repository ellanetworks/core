// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package validate

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type ExpectedPCODNS struct {
	// Empty skips IPv4 DNS validation.
	IPv4 string
	// Empty skips IPv6 DNS validation.
	IPv6 string
}

func PCODNS(plain []byte, expected *ExpectedPCODNS) error {
	if len(plain) < 4 {
		return fmt.Errorf("NAS message is too short")
	}

	var pco *nas.ProtocolConfigurationOptions

	switch fgs.GSMMessageType(plain[3]) {
	case fgs.MsgPDUSessionModificationCommand:
		cmd, err := fgs.ParsePDUSessionModificationCommand(plain)
		if err != nil {
			return fmt.Errorf("could not parse PDU Session Modification Command: %v", err)
		}

		if cmd.ExtendedPCO == nil {
			return fmt.Errorf("ExtendedProtocolConfigurationOptions is nil in PDU Session Modification Command")
		}

		pco = cmd.ExtendedPCO

	case fgs.MsgPDUSessionEstablishmentAccept:
		acc, err := fgs.ParsePDUSessionEstablishmentAccept(plain)
		if err != nil {
			return fmt.Errorf("could not parse PDU Session Establishment Accept: %v", err)
		}

		if acc.ExtendedPCO == nil {
			return fmt.Errorf("ExtendedProtocolConfigurationOptions is nil in PDU Session Establishment Accept")
		}

		pco = acc.ExtendedPCO

	default:
		return fmt.Errorf("message does not contain PCO: expected Modification Command or Establishment Accept")
	}

	dnsServers := pco.DNSServers()
	if len(dnsServers) == 0 {
		return fmt.Errorf("no DNS servers found in PCO")
	}

	foundIPv4 := false
	foundIPv6 := false

	for _, ip := range dnsServers {
		dns := ip.String()

		if ip.Is4() {
			foundIPv4 = true

			if expected.IPv4 != "" && dns != expected.IPv4 {
				return fmt.Errorf("IPv4 DNS mismatch: got %s, expected %s", dns, expected.IPv4)
			}
		} else {
			foundIPv6 = true

			if expected.IPv6 != "" && dns != expected.IPv6 {
				return fmt.Errorf("IPv6 DNS mismatch: got %s, expected %s", dns, expected.IPv6)
			}
		}
	}

	if expected.IPv4 != "" && !foundIPv4 {
		return fmt.Errorf("expected IPv4 DNS %s but none found in PCO", expected.IPv4)
	}

	if expected.IPv6 != "" && !foundIPv6 {
		return fmt.Errorf("expected IPv6 DNS %s but none found in PCO", expected.IPv6)
	}

	return nil
}
