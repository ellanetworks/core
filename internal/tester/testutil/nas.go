// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
)

func GetNasPduFromPduAccept(dlNas *nas.Message) (*nas.Message, error) {
	payload := dlNas.DLNASTransport.GetPayloadContainerContents()
	m := new(nas.Message)

	err := m.PlainNasDecode(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NAS PDU: %v", err)
	}

	return m, nil
}

func GetNASPDUFromDownlinkNasTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *ngapType.NASPDU {
	for _, ie := range downlinkNASTransport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDNASPDU:
			return ie.Value.NASPDU
		default:
			continue
		}
	}

	return nil
}

func GetAMFUENGAPIDFromDownlinkNASTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *ngapType.AMFUENGAPID {
	for _, ie := range downlinkNASTransport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			return ie.Value.AMFUENGAPID
		default:
			continue
		}
	}

	return nil
}

func UEIPFromNAS(pduSessionType uint8, ip [12]uint8) (netip.Addr, error) {
	switch pduSessionType {
	case 3: // IPv4v6 dual-stack: IPv4 is at bytes 8-11
		return netip.AddrFrom4([4]byte{ip[8], ip[9], ip[10], ip[11]}), nil
	case 2: // IPv6-only: no IPv4 address
		return netip.Addr{}, nil
	default: // IPv4-only: IPv4 is at bytes 0-3
		return netip.AddrFrom4([4]byte{ip[0], ip[1], ip[2], ip[3]}), nil
	}
}

func MTUFromExtendProtocolConfigurationOptionsContents(pco_buf []byte) (uint16, error) {
	pco := nasConvert.NewProtocolConfigurationOptions()

	err := pco.UnMarshal(pco_buf)
	if err != nil {
		return 0, fmt.Errorf("could not decode Extended Protocol Configuration Options: %v", err)
	}

	for _, o := range pco.ProtocolOrContainerList {
		switch o.ProtocolOrContainerID {
		case nasMessage.IPv4LinkMTUDL:
			return binary.BigEndian.Uint16(o.Contents), nil
		default:
			continue
		}
	}

	return 0, nil
}

func SDFromNAS(sd [3]uint8) string {
	return fmt.Sprintf("%x%x%x", sd[0], sd[1], sd[2])
}

// DNSFromExtendProtocolConfigurationOptionsContents extracts DNS server
// addresses from Extended Protocol Configuration Options contents.
// Returns (nil, nil) when no DNS servers are present.
func DNSFromExtendProtocolConfigurationOptionsContents(pco_buf []byte) ([]string, error) {
	pco := nasConvert.NewProtocolConfigurationOptions()

	err := pco.UnMarshal(pco_buf)
	if err != nil {
		return nil, fmt.Errorf("could not decode Extended Protocol Configuration Options: %v", err)
	}

	var dnsServers []string

	for _, o := range pco.ProtocolOrContainerList {
		switch o.ProtocolOrContainerID {
		case nasMessage.DNSServerIPv4AddressRequestUL:
			if len(o.Contents) >= 4 {
				dnsServers = append(dnsServers, fmt.Sprintf("%d.%d.%d.%d",
					o.Contents[0], o.Contents[1], o.Contents[2], o.Contents[3]))
			}
		case nasMessage.DNSServerIPv6AddressRequestUL:
			if len(o.Contents) >= 16 {
				var addr [16]byte
				copy(addr[:], o.Contents[:16])
				dnsServers = append(dnsServers, netip.AddrFrom16(addr).String())
			}
		default:
			continue
		}
	}

	return dnsServers, nil
}
