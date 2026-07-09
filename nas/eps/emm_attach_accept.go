// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// EPS attach result values (TS 24.301).
const (
	AttachResultEPS      uint8 = 1
	AttachResultCombined uint8 = 2
)

// AttachAccept is the ATTACH ACCEPT message (TS 24.301).
type AttachAccept struct {
	EPSAttachResult     uint8
	T3412               uint8
	TAIList             []byte
	ESMMessageContainer []byte
	GUTI                *EPSMobileIdentity // assigned GUTI (IEI 0x50), when present
	EMMCause            *uint8             // EMM cause (IEI 0x53), when present
	// EPS network feature support (IEI 0x64), when present (TS 24.301).
	EPSNetworkFeatureSupport *EPSNetworkFeatureSupport
}

// attachAcceptIEs are the optional IEs Ella Core emits in an ATTACH ACCEPT
// (TS 24.301): the assigned GUTI, the EMM cause, and the EPS network
// feature support. EMM cause is a type-3 IE with a one-octet value; the others
// are type-4 TLVs.
var attachAcceptIEs = []common.OptionalIE{
	{IEI: gutiIEI, Format: common.IETLV},
	{IEI: emmCauseIEI, Format: common.IETV3, Len: 1},
	{IEI: epsNetworkFeatureSupportIEI, Format: common.IETLV},
}

// EPSNetworkFeatureSupport is the EPS network feature support IE
// (TS 24.301), a type 4 IE whose content is 1 to 3 octets; only the
// first content octet is modelled. IMSVoPS is its bit 1 (IMS voice over PS
// session indicator), which the UE feeds to voice access-domain selection
// (TS 23.221). A UE that omits the higher octets reads them as zero.
type EPSNetworkFeatureSupport struct {
	IMSVoPS bool
}

func (n EPSNetworkFeatureSupport) encode() byte {
	var b byte
	if n.IMSVoPS {
		b |= 0x01
	}

	return b
}

// Marshal encodes the plain ATTACH ACCEPT message.
func (m *AttachAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAttachAccept)
	w.U8(m.EPSAttachResult & 0x07) // EPS attach result | spare half octet
	w.U8(m.T3412)

	if err := w.LV(m.TAIList); err != nil {
		return nil, err
	}

	if err := w.LVE(m.ESMMessageContainer); err != nil {
		return nil, err
	}

	if m.GUTI != nil {
		v, err := m.GUTI.encode()
		if err != nil {
			return nil, err
		}

		w.U8(gutiIEI)

		if err := w.LV(v); err != nil {
			return nil, err
		}
	}

	if m.EMMCause != nil {
		w.U8(emmCauseIEI)
		w.U8(*m.EMMCause)
	}

	if m.EPSNetworkFeatureSupport != nil {
		w.U8(epsNetworkFeatureSupportIEI)

		if err := w.LV([]byte{m.EPSNetworkFeatureSupport.encode()}); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseAttachAccept decodes a plain ATTACH ACCEPT message.
func ParseAttachAccept(b []byte) (*AttachAccept, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAttachAccept); err != nil {
		return nil, err
	}

	result, err := r.U8()
	if err != nil {
		return nil, err
	}

	t3412, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AttachAccept{EPSAttachResult: result & 0x07, T3412: t3412}

	if m.TAIList, err = r.LV(); err != nil {
		return nil, err
	}

	if m.ESMMessageContainer, err = r.LVE(); err != nil {
		return nil, err
	}

	if _, err := common.WalkOptionalIEs(r, attachAcceptIEs, func(iei uint8, value []byte) error {
		switch iei {
		case gutiIEI:
			id, err := decodeEPSMobileIdentity(value)
			if err != nil {
				return err
			}

			m.GUTI = &id
		case emmCauseIEI:
			if len(value) == 1 {
				c := value[0]
				m.EMMCause = &c
			}
		case epsNetworkFeatureSupportIEI:
			if len(value) >= 1 {
				m.EPSNetworkFeatureSupport = &EPSNetworkFeatureSupport{IMSVoPS: value[0]&0x01 != 0}
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// AttachComplete is the ATTACH COMPLETE message (TS 24.301).
type AttachComplete struct {
	ESMMessageContainer []byte
}

// Marshal encodes the plain ATTACH COMPLETE message.
func (m *AttachComplete) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAttachComplete)

	if err := w.LVE(m.ESMMessageContainer); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseAttachComplete decodes a plain ATTACH COMPLETE message.
func ParseAttachComplete(b []byte) (*AttachComplete, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAttachComplete); err != nil {
		return nil, err
	}

	esm, err := r.LVE()
	if err != nil {
		return nil, err
	}

	return &AttachComplete{ESMMessageContainer: esm}, nil
}

// AttachReject is the ATTACH REJECT message (TS 24.301). Ella Core sends the
// mandatory EMM cause and, when non-zero, the optional T3402 value (§8.2.3.4) —
// the back-off before the UE retries, mirroring the 5G T3502 in REGISTRATION
// REJECT.
type AttachReject struct {
	Cause uint8
	// T3402 is the encoded one-octet GPRS timer value (§9.9.3.16A); 0 omits the IE.
	T3402 uint8
}

// attachRejectIEs are the optional IEs Ella Core emits in an ATTACH REJECT: the
// T3402 value, a type-4 (TLV) "GPRS timer 2" IE with a one-octet value (§8.2.3.4).
var attachRejectIEs = []common.OptionalIE{
	{IEI: t3402ValueIEI, Format: common.IETLV},
}

// Marshal encodes the plain ATTACH REJECT message.
func (m *AttachReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAttachReject)
	w.U8(m.Cause)

	if m.T3402 != 0 {
		w.U8(t3402ValueIEI)

		if err := w.LV([]byte{m.T3402}); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseAttachReject decodes a plain ATTACH REJECT message.
func ParseAttachReject(b []byte) (*AttachReject, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAttachReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AttachReject{Cause: cause}

	if _, err := common.WalkOptionalIEs(r, attachRejectIEs, func(iei uint8, value []byte) error {
		if iei == t3402ValueIEI && len(value) == 1 {
			m.T3402 = value[0]
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
