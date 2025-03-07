package util

import (
	"fmt"
	"net"

	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapType"
)

func IPAddressToNgap(ipv4Addr, ipv6Addr string) (*ngapType.TransportLayerAddress, error) {
	if ipv4Addr == "" && ipv6Addr == "" {
		return nil, fmt.Errorf("both ipv4 and ipv6 are nil string")
	}
	var ipAddr *ngapType.TransportLayerAddress
	if ipv4Addr != "" && ipv6Addr != "" { // Both ipv4 & ipv6
		ipv4NetIP := net.ParseIP(ipv4Addr).To4()
		ipv6NetIP := net.ParseIP(ipv6Addr).To16()

		ipBytes := []byte{ipv4NetIP[0], ipv4NetIP[1], ipv4NetIP[2], ipv4NetIP[3]}
		for i := 0; i < 16; i++ {
			ipBytes = append(ipBytes, ipv6NetIP[i])
		}

		ipAddr.Value = aper.BitString{
			Bytes:     ipBytes,
			BitLength: 160,
		}
	} else if ipv4Addr != "" && ipv6Addr == "" { // ipv4
		ipv4NetIP := net.ParseIP(ipv4Addr).To4()

		ipBytes := []byte{ipv4NetIP[0], ipv4NetIP[1], ipv4NetIP[2], ipv4NetIP[3]}

		ipAddr.Value = aper.BitString{
			Bytes:     ipBytes,
			BitLength: 32,
		}
	} else { // ipv6
		ipv6NetIP := net.ParseIP(ipv6Addr).To16()

		ipBytes := []byte{}
		for i := 0; i < 16; i++ {
			ipBytes = append(ipBytes, ipv6NetIP[i])
		}

		ipAddr.Value = aper.BitString{
			Bytes:     ipBytes,
			BitLength: 128,
		}
	}

	return ipAddr, nil
}
