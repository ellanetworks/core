// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"

	nrppadec "github.com/ellanetworks/core/internal/decoder/nrppa"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
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

func buildDownlinkUEAssociatedNRPPaTransport(msg ngapType.DownlinkUEAssociatedNRPPaTransport) NGAPMessageValue {
	return buildUEAssociatedNRPPaTransportIEs(msg.ProtocolIEs.List)
}

func buildUplinkUEAssociatedNRPPaTransport(msg ngapType.UplinkUEAssociatedNRPPaTransport) NGAPMessageValue {
	ies := make([]IE, 0, len(msg.ProtocolIEs.List))

	for i := range msg.ProtocolIEs.List {
		ie := &msg.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ies = append(ies, nrppaAMFUENGAPIDIE(ie.Criticality.Value, ie.Value.AMFUENGAPID))
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, nrppaRANUENGAPIDIE(ie.Criticality.Value, ie.Value.RANUENGAPID))
		case ngapType.ProtocolIEIDRoutingID:
			ies = append(ies, nrppaRoutingIDIE(ie.Criticality.Value, ie.Value.RoutingID))
		case ngapType.ProtocolIEIDNRPPaPDU:
			ies = append(ies, nrppaPDUIE(ie.Criticality.Value, ie.Value.NRPPaPDU))
		default:
			ies = append(ies, unsupportedIE(ie.Id.Value, ie.Criticality.Value))
		}
	}

	return NGAPMessageValue{IEs: ies}
}

func buildUEAssociatedNRPPaTransportIEs(list []ngapType.DownlinkUEAssociatedNRPPaTransportIEs) NGAPMessageValue {
	ies := make([]IE, 0, len(list))

	for i := range list {
		ie := &list[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ies = append(ies, nrppaAMFUENGAPIDIE(ie.Criticality.Value, ie.Value.AMFUENGAPID))
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, nrppaRANUENGAPIDIE(ie.Criticality.Value, ie.Value.RANUENGAPID))
		case ngapType.ProtocolIEIDRoutingID:
			ies = append(ies, nrppaRoutingIDIE(ie.Criticality.Value, ie.Value.RoutingID))
		case ngapType.ProtocolIEIDNRPPaPDU:
			ies = append(ies, nrppaPDUIE(ie.Criticality.Value, ie.Value.NRPPaPDU))
		default:
			ies = append(ies, unsupportedIE(ie.Id.Value, ie.Criticality.Value))
		}
	}

	return NGAPMessageValue{IEs: ies}
}

func buildDownlinkNonUEAssociatedNRPPaTransport(msg ngapType.DownlinkNonUEAssociatedNRPPaTransport) NGAPMessageValue {
	ies := make([]IE, 0, len(msg.ProtocolIEs.List))

	for i := range msg.ProtocolIEs.List {
		ie := &msg.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRoutingID:
			ies = append(ies, nrppaRoutingIDIE(ie.Criticality.Value, ie.Value.RoutingID))
		case ngapType.ProtocolIEIDNRPPaPDU:
			ies = append(ies, nrppaPDUIE(ie.Criticality.Value, ie.Value.NRPPaPDU))
		default:
			ies = append(ies, unsupportedIE(ie.Id.Value, ie.Criticality.Value))
		}
	}

	return NGAPMessageValue{IEs: ies}
}

func buildUplinkNonUEAssociatedNRPPaTransport(msg ngapType.UplinkNonUEAssociatedNRPPaTransport) NGAPMessageValue {
	ies := make([]IE, 0, len(msg.ProtocolIEs.List))

	for i := range msg.ProtocolIEs.List {
		ie := &msg.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRoutingID:
			ies = append(ies, nrppaRoutingIDIE(ie.Criticality.Value, ie.Value.RoutingID))
		case ngapType.ProtocolIEIDNRPPaPDU:
			ies = append(ies, nrppaPDUIE(ie.Criticality.Value, ie.Value.NRPPaPDU))
		default:
			ies = append(ies, unsupportedIE(ie.Id.Value, ie.Criticality.Value))
		}
	}

	return NGAPMessageValue{IEs: ies}
}

// --- shared IE builders ---

func nrppaAMFUENGAPIDIE(crit aper.Enumerated, v *ngapType.AMFUENGAPID) IE {
	ie := IE{
		ID:          protocolIEIDToEnum(ngapType.ProtocolIEIDAMFUENGAPID),
		Criticality: criticalityToEnum(crit),
	}
	if v != nil {
		ie.Value = v.Value
	}

	return ie
}

func nrppaRANUENGAPIDIE(crit aper.Enumerated, v *ngapType.RANUENGAPID) IE {
	ie := IE{
		ID:          protocolIEIDToEnum(ngapType.ProtocolIEIDRANUENGAPID),
		Criticality: criticalityToEnum(crit),
	}
	if v != nil {
		ie.Value = v.Value
	}

	return ie
}

func nrppaRoutingIDIE(crit aper.Enumerated, v *ngapType.RoutingID) IE {
	ie := IE{
		ID:          protocolIEIDToEnum(ngapType.ProtocolIEIDRoutingID),
		Criticality: criticalityToEnum(crit),
	}
	if v != nil {
		ie.Value = hex.EncodeToString(v.Value)
	}

	return ie
}

func nrppaPDUIE(crit aper.Enumerated, v *ngapType.NRPPaPDU) IE {
	ie := IE{
		ID:          protocolIEIDToEnum(ngapType.ProtocolIEIDNRPPaPDU),
		Criticality: criticalityToEnum(crit),
	}
	if v != nil {
		ie.Value = decodeNRPPaPDU(v.Value)
	}

	return ie
}

func unsupportedIE(id int64, crit aper.Enumerated) IE {
	return IE{
		ID:          protocolIEIDToEnum(id),
		Criticality: criticalityToEnum(crit),
		Error:       "unsupported ie type",
	}
}
