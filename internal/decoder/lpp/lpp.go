// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package lpp provides a labeled, JSON-serializable decoder view of LPP
// messages (TS 37.355) for the radio-event inspector. It wraps the
// internal/lmf/lpp codec and renders enums as utils.EnumField so the UI
// shows "Label (value)", matching the NAS and NRPPa decoders.
package lpp

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	lppmodels "github.com/ellanetworks/core/internal/lmf/lpp/models"
	"github.com/free5gc/aper"
)

// PDU is the decoder view of an LPP message carried inside a NAS Transport
// payload container. It mirrors the NASPDU / NRPPaPDU wrapper pattern:
// the raw bytes are exposed as hex, and the decoded tree is embedded.
type PDU struct {
	Protocol string   `json:"protocol"` // always "LPP"
	RawHex   string   `json:"raw_hex"`
	Decoded  *Message `json:"decoded,omitempty"`
}

// Message is the decoded view of an LPP-Message.
type Message struct {
	TransactionID  byte                 `json:"transaction_id"`
	Initiator      utils.EnumField[int] `json:"initiator"`
	EndTransaction bool                 `json:"end_transaction"`
	BodyKind       utils.EnumField[int] `json:"body_kind"`

	Capabilities        *Capabilities        `json:"capabilities,omitempty"`
	LocationInformation *LocationInformation `json:"location_information,omitempty"`
	Error               string               `json:"error,omitempty"`
}

// Capabilities is the decoded view of a ProvideCapabilities message.
type Capabilities struct {
	GNSS []utils.EnumField[int] `json:"gnss,omitempty"`
}

// LocationInformation is the decoded view of a ProvideLocationInformation message.
type LocationInformation struct {
	Latitude  int32 `json:"latitude"`
	Longitude int32 `json:"longitude"`
	Altitude  int32 `json:"altitude"`
}

// Decode parses raw LPP APER bytes into the labeled view. On decode failure
// it returns a PDU with only Error set.
func Decode(raw []byte) *PDU {
	pdu := &PDU{
		Protocol: "LPP",
		RawHex:   hex.EncodeToString(raw),
	}

	decoded, err := lpp.DecodeLPPMessage(raw)
	if err != nil {
		pdu.Decoded = &Message{Error: err.Error()}
		return pdu
	}

	pdu.Decoded = mapMessage(decoded)

	return pdu
}

func mapMessage(d *lpp.DecodedMessage) *Message {
	out := &Message{
		TransactionID:  d.TransactionID,
		Initiator:      initiatorEnum(d.Initiator),
		EndTransaction: d.EndTransaction,
		BodyKind:       bodyKindEnum(d.BodyKind),
	}

	if d.ProvideCapabilities != nil {
		out.Capabilities = mapCapabilities(d.ProvideCapabilities)
	}

	if d.ProvideLocationInformation != nil {
		out.LocationInformation = mapLocationInformation(d.ProvideLocationInformation)
	}

	return out
}

func mapCapabilities(caps *lppmodels.ProvideLocationCapabilities) *Capabilities {
	if caps == nil {
		return nil
	}

	out := &Capabilities{}

	for _, gnssID := range caps.GNSSCapability.Supported() {
		var lpptypeID aper.Enumerated

		switch gnssID {
		case lppmodels.GnssIDGps:
			lpptypeID = lpptype.GnssIDGps
		case lppmodels.GnssIDSbas:
			lpptypeID = lpptype.GnssIDSbas
		case lppmodels.GnssIDQzss:
			lpptypeID = lpptype.GnssIDQzss
		case lppmodels.GnssIDGalileo:
			lpptypeID = lpptype.GnssIDGalileo
		case lppmodels.GnssIDGlonass:
			lpptypeID = lpptype.GnssIDGlonass
		case lppmodels.GnssIDBds:
			lpptypeID = lpptype.GnssIDBds
		case lppmodels.GnssIDNavic:
			lpptypeID = lpptype.GnssIDNavic
		default:
			continue
		}

		out.GNSS = append(out.GNSS, gnssIDEnum(lpptypeID))
	}

	return out
}

func mapLocationInformation(li *lppmodels.ProvideLocationInformation) *LocationInformation {
	if li == nil {
		return nil
	}

	r := li.GNSSPositionResult

	return &LocationInformation{
		Latitude:  r.Latitude,
		Longitude: r.Longitude,
		Altitude:  r.Altitude,
	}
}

// --- enum label helpers ---

func initiatorEnum(v aper.Enumerated) utils.EnumField[int] {
	switch v {
	case lpptype.InitiatorLocationServer:
		return utils.MakeEnum(int(v), "locationServer", false)
	case lpptype.InitiatorTargetDevice:
		return utils.MakeEnum(int(v), "targetDevice", false)
	default:
		return utils.MakeEnum(int(v), "", true)
	}
}

func bodyKindEnum(present int) utils.EnumField[int] {
	labels := map[int]string{
		lpptype.LPPMessageBodyC1PresentRequestCapabilities:        "RequestCapabilities",
		lpptype.LPPMessageBodyC1PresentProvideCapabilities:        "ProvideCapabilities",
		lpptype.LPPMessageBodyC1PresentRequestAssistanceData:      "RequestAssistanceData",
		lpptype.LPPMessageBodyC1PresentProvideAssistanceData:      "ProvideAssistanceData",
		lpptype.LPPMessageBodyC1PresentRequestLocationInformation: "RequestLocationInformation",
		lpptype.LPPMessageBodyC1PresentProvideLocationInformation: "ProvideLocationInformation",
		lpptype.LPPMessageBodyC1PresentAbort:                      "Abort",
		lpptype.LPPMessageBodyC1PresentError:                      "Error",
	}

	label, ok := labels[present]
	if !ok {
		label = ""
	}

	return utils.MakeEnum(present, label, !ok)
}

func gnssIDEnum(v aper.Enumerated) utils.EnumField[int] {
	labels := map[aper.Enumerated]string{
		lpptype.GnssIDGps:     "GPS",
		lpptype.GnssIDSbas:    "SBAS",
		lpptype.GnssIDQzss:    "QZSS",
		lpptype.GnssIDGalileo: "Galileo",
		lpptype.GnssIDGlonass: "GLONASS",
		lpptype.GnssIDBds:     "BeiDou",
		lpptype.GnssIDNavic:   "NavIC",
	}

	label, ok := labels[v]
	if !ok {
		label = ""
	}

	return utils.MakeEnum(int(v), label, !ok)
}
