package validate

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/tester/testutil"
	"github.com/free5gc/nas"
)

// ExpectedPCODNS describes expected DNS servers in an Extended PCO.
type ExpectedPCODNS struct {
	// IPv4 is the expected IPv4 DNS server address.
	// Set to "" to skip IPv4 DNS validation.
	IPv4 string

	// IPv6 is the expected IPv6 DNS server address.
	// Set to "" to skip IPv6 DNS validation.
	IPv6 string
}

// PCODNS validates DNS server addresses in an Extended Protocol Configuration
// Options IE inside a NAS message.
func PCODNS(msg *nas.Message, expected *ExpectedPCODNS) error {
	if msg == nil {
		return fmt.Errorf("NAS message is nil")
	}

	var pcoContents []byte

	switch {
	case msg.PDUSessionModificationCommand != nil:
		pco := msg.PDUSessionModificationCommand.ExtendedProtocolConfigurationOptions
		if pco == nil {
			return fmt.Errorf("ExtendedProtocolConfigurationOptions is nil in PDU Session Modification Command")
		}

		pcoContents = pco.GetExtendedProtocolConfigurationOptionsContents()

	case msg.PDUSessionEstablishmentAccept != nil:
		pco := msg.PDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions
		if pco == nil {
			return fmt.Errorf("ExtendedProtocolConfigurationOptions is nil in PDU Session Establishment Accept")
		}

		pcoContents = pco.GetExtendedProtocolConfigurationOptionsContents()

	default:
		return fmt.Errorf("message does not contain PCO: expected Modification Command or Establishment Accept")
	}

	if len(pcoContents) == 0 {
		return fmt.Errorf("PCO contents is empty")
	}

	dnsServers, err := testutil.DNSFromExtendProtocolConfigurationOptionsContents(pcoContents)
	if err != nil {
		return fmt.Errorf("could not parse DNS from PCO: %v", err)
	}

	if len(dnsServers) == 0 {
		return fmt.Errorf("no DNS servers found in PCO")
	}

	foundIPv4 := false
	foundIPv6 := false

	for _, dns := range dnsServers {
		ip := net.ParseIP(dns)
		if ip == nil {
			continue
		}

		if ip.To4() != nil {
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
