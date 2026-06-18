// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type ESMHeader struct {
	MessageType                  utils.EnumField[uint64] `json:"message_type"`
	EPSBearerIdentity            uint8                   `json:"eps_bearer_identity"`
	ProcedureTransactionIdentity uint8                   `json:"procedure_transaction_identity"`
}

// ESMMessage is a decoded ESM message: its header, plus the salient fields of
// the session-management messages the MME exchanges. Unlisted types decode to
// the header only.
type ESMMessage struct {
	ESMHeader ESMHeader `json:"esm_header"`
	Error     string    `json:"error,omitempty"`

	PDNConnectivityRequest *PDNConnectivityRequest `json:"pdn_connectivity_request,omitempty"`
	ActivateDefaultBearer  *ActivateDefaultBearer  `json:"activate_default_bearer,omitempty"`
}

type PDNConnectivityRequest struct {
	RequestType utils.EnumField[uint64] `json:"request_type"`
	PDNType     utils.EnumField[uint64] `json:"pdn_type"`
}

type ActivateDefaultBearer struct {
	AccessPointName string      `json:"access_point_name,omitempty"`
	PDNAddress      *PDNAddress `json:"pdn_address,omitempty"`
}

// PDNAddress is the decoded PDN address IE (TS 24.301 §9.9.4.9): the assigned UE
// address. For IPv6 the network assigns only the 64-bit interface identifier
// (the prefix arrives via Router Advertisement / PCO).
type PDNAddress struct {
	Type            utils.EnumField[uint64] `json:"type"`
	IPv4            string                  `json:"ipv4,omitempty"`
	IPv6InterfaceID string                  `json:"ipv6_interface_id,omitempty"`
}

func buildESMMessage(b []byte) *ESMMessage {
	if len(b) < 3 {
		return &ESMMessage{Error: "ESM message too short"}
	}

	mt := eps.ESMMessageType(b[2])
	m := &ESMMessage{ESMHeader: ESMHeader{
		MessageType:                  esmTypeToEnum(mt),
		EPSBearerIdentity:            b[0] >> 4,
		ProcedureTransactionIdentity: b[1],
	}}

	switch mt {
	case eps.MsgPDNConnectivityRequest:
		if req, err := eps.ParsePDNConnectivityRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.PDNConnectivityRequest = &PDNConnectivityRequest{
				RequestType: requestTypeToEnum(req.RequestType),
				PDNType:     pdnTypeToEnum(req.PDNType),
			}
		}
	case eps.MsgActivateDefaultEPSBearerContextRequest:
		if req, err := eps.ParseActivateDefaultEPSBearerContextRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.ActivateDefaultBearer = &ActivateDefaultBearer{
				AccessPointName: decodeAPN(req.AccessPointName),
				PDNAddress:      pdnAddress(req.PDNAddress),
			}
		}
	}

	return m
}

// decodeESMContainer decodes an ESM message carried inside an EMM message (e.g.
// the PDN Connectivity Request in Attach Request). An empty container yields nil.
func decodeESMContainer(b []byte) *ESMMessage {
	if len(b) == 0 {
		return nil
	}

	return buildESMMessage(b)
}

func requestTypeToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{1: "initial request", 2: "handover", 3: "unused", 4: "emergency"}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

func pdnTypeToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{1: "IPv4", 2: "IPv6", 3: "IPv4v6"}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

// pdnAddress decodes the PDN address IE into the assigned UE address (TS 24.301
// §9.9.4.9). An undecodable value yields nil.
func pdnAddress(b []byte) *PDNAddress {
	if len(b) == 0 {
		return nil
	}

	addr, err := eps.ParsePDNAddress(b)
	if err != nil {
		return &PDNAddress{Type: utils.MakeEnum(uint64(0), hex.EncodeToString(b), true)}
	}

	out := &PDNAddress{Type: pdnTypeToEnum(addr.PDNType)}

	switch addr.PDNType {
	case 1: // IPv4
		out.IPv4 = net.IP(addr.IPv4[:]).String()
	case 2: // IPv6 (interface identifier only)
		out.IPv6InterfaceID = interfaceID(addr.IPv6IID)
	case 3: // IPv4v6
		out.IPv4 = net.IP(addr.IPv4[:]).String()
		out.IPv6InterfaceID = interfaceID(addr.IPv6IID)
	}

	return out
}

// interfaceID renders a 64-bit IPv6 interface identifier in colon-separated hex.
func interfaceID(iid [8]byte) string {
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x",
		iid[0], iid[1], iid[2], iid[3], iid[4], iid[5], iid[6], iid[7])
}

// decodeAPN renders an APN, whose value is a sequence of length-prefixed labels
// (TS 23.003 §9.1). A non-printable value falls back to hex.
func decodeAPN(b []byte) string {
	var labels []byte

	for i := 0; i < len(b); {
		n := int(b[i])
		i++

		if n == 0 || i+n > len(b) {
			return hex.EncodeToString(b)
		}

		if len(labels) > 0 {
			labels = append(labels, '.')
		}

		labels = append(labels, b[i:i+n]...)
		i += n
	}

	return string(labels)
}
