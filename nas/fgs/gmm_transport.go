// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Payload container type values (TS 24.501 §9.11.3.40).
const (
	PayloadContainerTypeN1SMInfo          uint8 = 0x01
	PayloadContainerTypeSMS               uint8 = 0x02
	PayloadContainerTypeLPP               uint8 = 0x03
	PayloadContainerTypeSOR               uint8 = 0x04
	PayloadContainerTypeUEPolicy          uint8 = 0x05
	PayloadContainerTypeUEParameterUpdate uint8 = 0x06
	PayloadContainerTypeMultiplePayload   uint8 = 0x0F
)

// UL NAS TRANSPORT request type values (TS 24.501 §9.11.3.47).
const (
	ULNASTransportRequestTypeInitialRequest              uint8 = 1
	ULNASTransportRequestTypeExistingPduSession          uint8 = 2
	ULNASTransportRequestTypeInitialEmergencyRequest     uint8 = 3
	ULNASTransportRequestTypeExistingEmergencyPduSession uint8 = 4
)

// DNNToString decodes a DNN IE value (RFC 1035 labels) into its dotted form.
func DNNToString(v []byte) string { return labelsToDNN(v) }

// DLNASTransport is the DL NAS TRANSPORT message (TS 24.501 §8.2.11): it carries
// a payload container (typically an SMF-produced 5GSM message) from the network
// to the UE, with optional routing and diagnostic IEs.
type DLNASTransport struct {
	PayloadContainerType uint8  // bits 1-4 (TS 24.501 §9.11.3.40)
	PayloadContainer     []byte // LV-E
	PDUSessionID         uint8  // optional (IEI 0x12); 0 omits the IE
	AdditionalInfo       []byte // optional (IEI 0x24)
	Cause                *uint8 // optional 5GMM cause (IEI 0x58)
}

// Marshal encodes the plain DL NAS TRANSPORT message.
func (m *DLNASTransport) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgDLNASTransport)
	w.U8(m.PayloadContainerType & 0x0F) // spare half octet in bits 5-8

	if err := w.LVE(m.PayloadContainer); err != nil {
		return nil, err
	}

	if m.PDUSessionID != 0 {
		writeTV2(&w, ieiPduSessionID2, m.PDUSessionID)
	}

	if len(m.AdditionalInfo) > 0 {
		writeTLV(&w, ieiAdditionalInfo, m.AdditionalInfo)
	}

	if m.Cause != nil {
		writeTV2(&w, ieiCause5GMM, *m.Cause)
	}

	return w.Bytes(), nil
}

// ULNASTransport is the UL NAS TRANSPORT message (TS 24.501 §8.2.10): the UE
// tunnels a payload container (typically a 5GSM message) to the network, with
// optional routing IEs identifying the PDU session, slice and DNN.
type ULNASTransport struct {
	PayloadContainerType  uint8  // bits 1-4 (TS 24.501 §9.11.3.40)
	PayloadContainer      []byte // LV-E
	PDUSessionID          *uint8 // optional (IEI 0x12)
	OldPDUSessionID       *uint8 // optional (IEI 0x59)
	RequestType           *uint8 // optional (IEI 0x8-, type 1): bits 1-3 (TS 24.501 §9.11.3.47)
	SNSSAI                []byte // optional (IEI 0x22)
	DNN                   []byte // optional (IEI 0x25)
	AdditionalInformation []byte // optional (IEI 0x24)
}

var ulNASTransportIEs = []common.OptionalIE{
	{IEI: ieiPduSessionID2, Format: common.IETV3, Len: 1},
	{IEI: ieiOldPDUSessionID, Format: common.IETV3, Len: 1},
	{IEI: ieiSNSSAI, Format: common.IETLV},
	{IEI: ieiDNN, Format: common.IETLV},
	{IEI: ieiAdditionalInfo, Format: common.IETLV},
}

// ParseULNASTransport decodes a plain UL NAS TRANSPORT message.
func ParseULNASTransport(b []byte) (*ULNASTransport, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgULNASTransport); err != nil {
		return nil, err
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	pc, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &ULNASTransport{
		PayloadContainerType: octet1 & 0x0F,
		PayloadContainer:     pc,
	}

	if _, err := common.WalkOptionalIEs(r, ulNASTransportIEs, func(iei uint8, value []byte) error {
		switch iei {
		case ieiPduSessionID2:
			if len(value) > 0 {
				v := value[0]
				out.PDUSessionID = &v
			}
		case ieiOldPDUSessionID:
			if len(value) > 0 {
				v := value[0]
				out.OldPDUSessionID = &v
			}
		case 0x80: // request type (type 1): value is the low nibble
			if len(value) > 0 {
				v := value[0] & 0x07
				out.RequestType = &v
			}
		case ieiSNSSAI:
			out.SNSSAI = value
		case ieiDNN:
			out.DNN = value
		case ieiAdditionalInfo:
			out.AdditionalInformation = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}
