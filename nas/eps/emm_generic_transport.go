// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// Generic NAS Transport carries an opaque container (e.g. an LPP positioning
// message) between the MME and the UE over EMM signalling (TS 24.301 §8.2.20
// downlink, §8.2.31 uplink). It is the LTE equivalent of the 5GS DL/UL NAS
// Transport used for LPP over N1.
const (
	MsgDLGenericNASTransport MessageType = 0x68
	MsgULGenericNASTransport MessageType = 0x69
)

// GenericContainerTypeLPP is the "generic message container type" value for an
// LTE Positioning Protocol (LPP) message (TS 24.301 §9.9.3.42).
const GenericContainerTypeLPP uint8 = 0x01

// additionalInformationIEI is the IEI of the optional Additional information IE
// (TS 24.301 §9.9.2.0), a type-4 TLV that carries the LCS correlation identifier
// so a positioning reply can be routed back to the right session.
const additionalInformationIEI uint8 = 0x65

// DLGenericNASTransport is the DOWNLINK GENERIC NAS TRANSPORT message
// (TS 24.301 §8.2.20): the MME transports Container (of type ContainerType) to
// the UE. AdditionalInfo is optional (nil = absent).
type DLGenericNASTransport struct {
	ContainerType  uint8
	Container      []byte
	AdditionalInfo []byte
}

// Marshal encodes the plain DOWNLINK GENERIC NAS TRANSPORT message.
func (m *DLGenericNASTransport) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgDLGenericNASTransport)
	w.U8(m.ContainerType)

	// Generic message container is a type-6 IE: a 2-octet length precedes the
	// value, so containers longer than 255 octets are carried.
	if err := w.LVE(m.Container); err != nil {
		return nil, err
	}

	if len(m.AdditionalInfo) > 0 {
		w.U8(additionalInformationIEI)

		if err := w.LV(m.AdditionalInfo); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ULGenericNASTransport is the UPLINK GENERIC NAS TRANSPORT message
// (TS 24.301 §8.2.31): the UE transports Container to the MME.
type ULGenericNASTransport struct {
	ContainerType  uint8
	Container      []byte
	AdditionalInfo []byte
}

// ParseULGenericNASTransport decodes a plain UPLINK GENERIC NAS TRANSPORT
// message.
func ParseULGenericNASTransport(b []byte) (*ULGenericNASTransport, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgULGenericNASTransport); err != nil {
		return nil, err
	}

	ct, err := r.U8()
	if err != nil {
		return nil, err
	}

	container, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &ULGenericNASTransport{ContainerType: ct, Container: container}

	// Optional IEs follow as TLVs. Only the Additional information IE is read;
	// any other optional IE is skipped as a length-value pair.
	for r.Remaining() > 0 {
		iei, err := r.PeekU8()
		if err != nil {
			break
		}

		if _, err := r.U8(); err != nil {
			break
		}

		v, err := r.LV()
		if err != nil {
			break
		}

		if iei == additionalInformationIEI {
			out.AdditionalInfo = v
		}
	}

	return out, nil
}
