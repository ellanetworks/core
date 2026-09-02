// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

// PCOContainerName returns the name TS 24.008 §10.5.6.3 gives a container in the
// given direction. The identifier is direction-scoped: 000DH is a DNS server
// address request uplink and the address itself downlink, so a reader that names
// it from the identifier alone would mislabel half of them. It returns the empty
// string for a reserved identifier, one the table does not name, and for a
// direction that is unset.
func PCOContainerName(id uint16, dir PCODirection) string {
	switch dir {
	case PCOMSToNetwork:
		return pcoUplinkContainerName(id)
	case PCONetworkToMS:
		return pcoDownlinkContainerName(id)
	default:
		return ""
	}
}

func pcoUplinkContainerName(id uint16) string {
	switch id {
	case 0x0001:
		return "P-CSCF IPv6 Address Request"
	case 0x0002:
		return "IM CN Subsystem Signaling Flag"
	case 0x0003:
		return "DNS Server IPv6 Address Request"
	case 0x0005:
		return "MS Support of Network Requested Bearer Control indicator"
	case 0x0007:
		return "DSMIPv6 Home Agent Address Request"
	case 0x0008:
		return "DSMIPv6 Home Network Prefix Request"
	case 0x0009:
		return "DSMIPv6 IPv4 Home Agent Address Request"
	case 0x000a:
		return "IP address allocation via NAS signalling"
	case 0x000b:
		return "IPv4 address allocation via DHCPv4"
	case 0x000c:
		return "P-CSCF IPv4 Address Request"
	case 0x000d:
		return "DNS Server IPv4 Address Request"
	case 0x000e:
		return "MSISDN Request"
	case 0x000f:
		return "IFOM-Support-Request"
	case 0x0010:
		return "IPv4 Link MTU Request"
	case 0x0011:
		return "MS support of Local address in TFT indicator"
	case 0x0012:
		return "P-CSCF Re-selection support"
	case 0x0013:
		return "NBIFOM request indicator"
	case 0x0014:
		return "NBIFOM mode"
	case 0x0015:
		return "Non-IP Link MTU Request"
	case 0x0016:
		return "APN rate control support indicator"
	case 0x0017:
		return "3GPP PS data off UE status"
	case 0x0018:
		return "Reliable Data Service request indicator"
	case 0x0019:
		return "Additional APN rate control for exception data support indicator"
	case 0x001a:
		return "PDU session ID"
	case 0x0020:
		return "Ethernet Frame Payload MTU Request"
	case 0x0021:
		return "Unstructured Link MTU Request"
	case 0x0022:
		return "5GSM cause value"
	default:
		return protocolContainerName(id)
	}
}

func pcoDownlinkContainerName(id uint16) string {
	switch id {
	case 0x0001:
		return "P-CSCF IPv6 Address"
	case 0x0002:
		return "IM CN Subsystem Signaling Flag"
	case 0x0003:
		return "DNS Server IPv6 Address"
	case 0x0004:
		return "Policy Control rejection code"
	case 0x0005:
		return "Selected Bearer Control Mode"
	case 0x0007:
		return "DSMIPv6 Home Agent Address"
	case 0x0008:
		return "DSMIPv6 Home Network Prefix"
	case 0x0009:
		return "DSMIPv6 IPv4 Home Agent Address"
	case 0x000c:
		return "P-CSCF IPv4 Address"
	case 0x000d:
		return "DNS Server IPv4 Address"
	case 0x000e:
		return "MSISDN"
	case 0x000f:
		return "IFOM-Support"
	case 0x0010:
		return "IPv4 Link MTU"
	case 0x0011:
		return "Network support of Local address in TFT indicator"
	case 0x0013:
		return "NBIFOM accepted indicator"
	case 0x0014:
		return "NBIFOM mode"
	case 0x0015:
		return "Non-IP Link MTU"
	case 0x0016:
		return "APN rate control parameters"
	case 0x0017:
		return "3GPP PS data off support indication"
	case 0x0018:
		return "Reliable Data Service accepted indicator"
	case 0x0019:
		return "Additional APN rate control for exception data parameters"
	case 0x001b:
		return "S-NSSAI"
	case 0x001c:
		return "QoS rules"
	case 0x001d:
		return "Session-AMBR"
	case 0x001e:
		return "PDU session address lifetime"
	case 0x001f:
		return "QoS flow descriptions"
	case 0x0020:
		return "Ethernet Frame Payload MTU"
	case 0x0021:
		return "Unstructured Link MTU"
	case 0x0023:
		return "QoS rules with the length of two octets"
	default:
		return protocolContainerName(id)
	}
}

// protocolContainerName names the containers that carry another protocol, which
// both directions share (TS 24.008 §10.5.6.3).
func protocolContainerName(id uint16) string {
	switch id {
	case 0x8021:
		return "IPCP"
	case 0xc021:
		return "LCP"
	case 0xc023:
		return "PAP"
	case 0xc223:
		return "CHAP"
	default:
		return ""
	}
}
