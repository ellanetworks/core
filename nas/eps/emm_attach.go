// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// IEIs of the ATTACH REQUEST optional information elements that the message
// orders at or before the MS network capability (TS 24.301).
const (
	oldPTMSISignatureIEI        = 0x19 // TV, 3-octet value
	additionalGUTIIEI           = 0x50 // TLV
	lastVisitedRegisteredTAIIEI = 0x52 // TV, 5-octet value
	drxParameterIEI             = 0x5C // TV, 2-octet value
	msNetworkCapabilityIEI      = 0x31 // TLV
)

// attachRequestIEs are the optional IEs the ATTACH REQUEST orders at or before
// the MS network capability (TS 24.301). The walker delimits them so the
// MS network capability is read correctly regardless of which precede it; IEs
// that follow it are not consumed by Ella Core.
var attachRequestIEs = []common.OptionalIE{
	{IEI: oldPTMSISignatureIEI, Format: common.IETV3, Len: 3},
	{IEI: additionalGUTIIEI, Format: common.IETLV},
	{IEI: lastVisitedRegisteredTAIIEI, Format: common.IETV3, Len: 5},
	{IEI: drxParameterIEI, Format: common.IETV3, Len: 2},
	{IEI: msNetworkCapabilityIEI, Format: common.IETLV},
}

// AttachRequest is the ATTACH REQUEST message (TS 24.301).
// MSNetworkCapability is decoded from the optional part for the GERAN security
// capabilities the SECURITY MODE COMMAND replays.
type AttachRequest struct {
	EPSAttachType       uint8
	NASKeySetIdentifier uint8
	EPSMobileIdentity   EPSMobileIdentity
	UENetworkCapability []byte
	ESMMessageContainer []byte
	MSNetworkCapability []byte
}

// Marshal encodes the plain ATTACH REQUEST message.
func (m *AttachRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAttachRequest)
	w.U8(m.NASKeySetIdentifier<<4 | m.EPSAttachType&0x07)

	mobid, err := m.EPSMobileIdentity.encode()
	if err != nil {
		return nil, err
	}

	if err := w.LV(mobid); err != nil {
		return nil, err
	}

	if err := w.LV(m.UENetworkCapability); err != nil {
		return nil, err
	}

	if err := w.LVE(m.ESMMessageContainer); err != nil {
		return nil, err
	}

	if len(m.MSNetworkCapability) > 0 {
		w.U8(msNetworkCapabilityIEI)

		if err := w.LV(m.MSNetworkCapability); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseAttachRequest decodes a plain ATTACH REQUEST message.
func ParseAttachRequest(b []byte) (*AttachRequest, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAttachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AttachRequest{EPSAttachType: octet & 0x07, NASKeySetIdentifier: octet >> 4}

	mobid, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSMobileIdentity, err = decodeEPSMobileIdentity(mobid); err != nil {
		return nil, err
	}

	if m.UENetworkCapability, err = r.LV(); err != nil {
		return nil, err
	}

	if m.ESMMessageContainer, err = r.LVE(); err != nil {
		return nil, err
	}

	if _, err := common.WalkOptionalIEs(r, attachRequestIEs, func(iei uint8, value []byte) error {
		if iei == msNetworkCapabilityIEI {
			m.MSNetworkCapability = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
