// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	nrppadec "github.com/ellanetworks/core/internal/decoder/nrppa"
	"github.com/ellanetworks/core/ngap"
)

// NRPPaPDU is the decoder view of an NRPPa PDU carried inside an NGAP
// UE-associated / non-UE-associated NRPPa transport message. It mirrors the
// NASPDU wrapper: the raw octet string is exposed as hex, and the decoded
// tree (E-CID Measurement Initiation procedures, TS 38.455) is embedded.
type NRPPaPDU struct {
	Protocol string            `json:"protocol"`
	RawHex   string            `json:"raw_hex"`
	Decoded  *nrppadec.Message `json:"decoded,omitempty"`
}

// decodeNRPPaPDU builds the NRPPaPDU wrapper from a raw octet string.
func decodeNRPPaPDU(raw []byte) NRPPaPDU {
	return NRPPaPDU{
		Protocol: "NRPPa",
		RawHex:   hex.EncodeToString(raw),
		Decoded:  nrppadec.Decode(raw),
	}
}

// ueAssociatedNRPPaIEs renders the four IEs both UE-associated transports
// carry; they differ only in procedure code (TS 38.413 §9.2.9.1, §9.2.9.2).
func ueAssociatedNRPPaIEs(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, routing ngap.RoutingID, pdu ngap.NRPPaPDU, unknown []ngap.RawIE) NGAPMessageValue {
	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(amfID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(ranID)),
		ie(ngap.IDRoutingID, ngap.CriticalityReject, hex.EncodeToString(routing)),
		ie(ngap.IDNRPPaPDU, ngap.CriticalityReject, decodeNRPPaPDU(pdu)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(unknown)...)}
}

// nonUEAssociatedNRPPaIEs renders the two IEs both non-UE-associated transports
// carry (TS 38.413 §9.2.9.3, §9.2.9.4).
func nonUEAssociatedNRPPaIEs(routing ngap.RoutingID, pdu ngap.NRPPaPDU, unknown []ngap.RawIE) NGAPMessageValue {
	ies := []IE{
		ie(ngap.IDRoutingID, ngap.CriticalityReject, hex.EncodeToString(routing)),
		ie(ngap.IDNRPPaPDU, ngap.CriticalityReject, decodeNRPPaPDU(pdu)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(unknown)...)}
}

func buildDownlinkUEAssociatedNRPPaTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkUEAssociatedNRPPaTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink UE-associated NRPPa Transport: %v", err)}
	}

	return ueAssociatedNRPPaIEs(m.AMFUENGAPID, m.RANUENGAPID, m.RoutingID, m.NRPPaPDU, m.UnknownIEs())
}

func buildUplinkUEAssociatedNRPPaTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUplinkUEAssociatedNRPPaTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Uplink UE-associated NRPPa Transport: %v", err)}
	}

	return ueAssociatedNRPPaIEs(m.AMFUENGAPID, m.RANUENGAPID, m.RoutingID, m.NRPPaPDU, m.UnknownIEs())
}

func buildDownlinkNonUEAssociatedNRPPaTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseDownlinkNonUEAssociatedNRPPaTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Downlink non-UE-associated NRPPa Transport: %v", err)}
	}

	return nonUEAssociatedNRPPaIEs(m.RoutingID, m.NRPPaPDU, m.UnknownIEs())
}

func buildUplinkNonUEAssociatedNRPPaTransport(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUplinkNonUEAssociatedNRPPaTransport(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Uplink non-UE-associated NRPPa Transport: %v", err)}
	}

	return nonUEAssociatedNRPPaIEs(m.RoutingID, m.NRPPaPDU, m.UnknownIEs())
}
