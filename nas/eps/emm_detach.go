// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// Type of detach values (TS 24.301 §9.9.3.7). For UE-originating detach:
// 1 = EPS detach, 2 = IMSI detach, 3 = combined EPS/IMSI detach.
const (
	DetachTypeEPS      uint8 = 1
	DetachTypeIMSI     uint8 = 2
	DetachTypeCombined uint8 = 3
)

// ieiEMMCause is the IEI of the optional EMM cause in a network-originating
// DETACH REQUEST (TS 24.301 §8.2.12). EMM cause is a type-3 IE with a one-octet
// value (TS 24.301 §9.9.3.9).
const ieiEMMCause uint8 = 0x53

// detachRequestNetworkIEs are the optional IEs of a network-originating DETACH
// REQUEST (TS 24.301 §8.2.12): the EMM cause.
var detachRequestNetworkIEs = []common.OptionalIE{
	{IEI: ieiEMMCause, Format: common.IETV3, Len: 1},
}

// DetachRequestUE is the UE-originating DETACH REQUEST message (TS 24.301
// §8.2.11). SwitchOff indicates the UE is powering off, in which case the
// network does not send a Detach Accept.
type DetachRequestUE struct {
	SwitchOff           bool
	TypeOfDetach        uint8
	NASKeySetIdentifier uint8
	EPSMobileIdentity   EPSMobileIdentity
}

// Marshal encodes the plain UE-originating DETACH REQUEST message.
func (m *DetachRequestUE) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgDetachRequest)

	octet := m.NASKeySetIdentifier<<4 | m.TypeOfDetach&0x07
	if m.SwitchOff {
		octet |= 0x08
	}

	w.U8(octet)

	mobid, err := m.EPSMobileIdentity.encode()
	if err != nil {
		return nil, err
	}

	if err := w.LV(mobid); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseDetachRequestUE decodes a plain UE-originating DETACH REQUEST message.
func ParseDetachRequestUE(b []byte) (*DetachRequestUE, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgDetachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &DetachRequestUE{
		SwitchOff:           octet&0x08 != 0,
		TypeOfDetach:        octet & 0x07,
		NASKeySetIdentifier: octet >> 4,
	}

	mobid, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSMobileIdentity, err = decodeEPSMobileIdentity(mobid); err != nil {
		return nil, err
	}

	return m, nil
}

// DetachRequestNetwork is the network-originating DETACH REQUEST message
// (TS 24.301 §8.2.12). EMMCause is nil when the optional cause is absent.
type DetachRequestNetwork struct {
	TypeOfDetach uint8
	EMMCause     *uint8
}

// Marshal encodes the plain network-originating DETACH REQUEST message.
func (m *DetachRequestNetwork) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgDetachRequest)
	w.U8(m.TypeOfDetach & 0x07) // detach type | spare half octet

	if m.EMMCause != nil {
		w.U8(ieiEMMCause)
		w.U8(*m.EMMCause)
	}

	return w.Bytes(), nil
}

// ParseDetachRequestNetwork decodes a plain network-originating DETACH REQUEST
// message.
func ParseDetachRequestNetwork(b []byte) (*DetachRequestNetwork, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgDetachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &DetachRequestNetwork{TypeOfDetach: octet & 0x07}

	if _, err := common.WalkOptionalIEs(r, detachRequestNetworkIEs, func(iei uint8, value []byte) error {
		if iei == ieiEMMCause && len(value) == 1 {
			cause := value[0]
			m.EMMCause = &cause
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// DetachAccept is the DETACH ACCEPT message (TS 24.301 §8.2.10), used in both
// directions; it has no information elements beyond the header.
type DetachAccept struct{}

// Marshal encodes the plain DETACH ACCEPT message.
func (m *DetachAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgDetachAccept)

	return w.Bytes(), nil
}

// ParseDetachAccept decodes a plain DETACH ACCEPT message.
func ParseDetachAccept(b []byte) (*DetachAccept, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgDetachAccept); err != nil {
		return nil, err
	}

	return &DetachAccept{}, nil
}
