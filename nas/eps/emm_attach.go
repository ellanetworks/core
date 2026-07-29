// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// attachRequestIEs are the full-octet optional IEs of the ATTACH REQUEST, in the
// order TS 24.301 Table 8.2.4.1 lists them, so the walker delimits every one and
// no IE hides those after it. Type-1 IEs (IEI ≥ 0x80) are delimited generically
// and are not listed.
var attachRequestIEs = []nas.OptionalIE{
	{IEI: ieiOldPTMSISignature, Format: nas.IETV3, Len: 3, Name: "Old P-TMSI signature"},
	{IEI: ieiAdditionalGUTI, Format: nas.IETLV, Name: "Additional GUTI"},
	{IEI: ieiLastVisitedRegisteredTAI, Format: nas.IETV3, Len: 5, Name: "Last visited registered TAI"},
	{IEI: ieiDRXParameter, Format: nas.IETV3, Len: 2, Name: "DRX parameter"},
	{IEI: ieiMSNetworkCapability, Format: nas.IETLV, Critical: true, Name: "MS network capability"},
	{IEI: ieiOldLocationAreaID, Format: nas.IETV3, Len: 5, Name: "Old location area identification"},
	{IEI: ieiMobileStationClassmark2, Format: nas.IETLV, Name: "Mobile station classmark 2"},
	{IEI: ieiMobileStationClassmark3, Format: nas.IETLV, Name: "Mobile station classmark 3"},
	{IEI: ieiSupportedCodecs, Format: nas.IETLV, Name: "Supported codecs"},
	{IEI: ieiVoiceDomainPreference, Format: nas.IETLV, Name: "Voice domain preference"},
	{IEI: ieiTMSIBasedNRIContainer, Format: nas.IETLV, Name: "TMSI based NRI container"},
	{IEI: ieiT3324Value, Format: nas.IETLV, Name: "T3324 value"},
	{IEI: ieiT3412ExtendedValue, Format: nas.IETLV, Name: "T3412 extended value"},
	{IEI: ieiExtendedDRXParameters, Format: nas.IETLV, Name: "Extended DRX parameters"},
	{IEI: ieiUEAdditionalSecurityCap, Format: nas.IETLV, Name: "UE additional security capability"},
	{IEI: ieiUEStatus, Format: nas.IETLV, Name: "UE status"},
	{IEI: ieiAdditionalInformationRequested, Format: nas.IETV3, Len: 1, Name: "Additional information requested"},
	{IEI: ieiN1UENetworkCapability, Format: nas.IETLV, Name: "N1 UE network capability"},
	{IEI: ieiUERadioCapabilityIDAvail, Format: nas.IETLV, Name: "UE radio capability ID availability"},
	{IEI: ieiRequestedWUSAssistance, Format: nas.IETLV, Name: "Requested WUS assistance"},
	{IEI: ieiDRXParameterNBS1Mode, Format: nas.IETLV, Name: "DRX parameter in NB-S1 mode"},
	{IEI: ieiRequestedIMSIOffset, Format: nas.IETLV, Name: "Requested IMSI offset"},
}

// AttachRequest is the ATTACH REQUEST message (TS 24.301 §8.2.4). The optional
// IEs are carried as their raw values, not decoded further: a nil slice (or nil
// pointer, for the type-1 half-octet IEs) means the UE omitted the IE. The values
// round-trip byte-for-byte through MarshalBinary.
type AttachRequest struct {
	EPSAttachType       AttachType
	NASKeySetIdentifier nas.KeySetIdentifier
	EPSMobileIdentity   EPSMobileIdentity
	UENetworkCapability UENetworkCapability
	ESMMessageContainer []byte

	AdditionalGUTI      *EPSMobileIdentity   // IEI 0x50, §8.2.4.3
	MSNetworkCapability *MSNetworkCapability // IEI 0x31, §8.2.4.6
	// TMSIStatus reports whether the UE holds a valid TMSI (IEI 0x9-, §8.2.4.8).
	TMSIStatus           *bool
	AdditionalUpdateType *AdditionalUpdateType // IEI 0xF-, §8.2.4.12
	// DeviceProperties reports whether the UE is configured for low priority
	// (IEI 0xD-, §8.2.4.14).
	DeviceProperties *bool
	OldGUTIType      *GUTIType // IEI 0xE-, §8.2.4.15
	// MSNetworkFeatureSupport reports whether the UE supports the extended
	// periodic timer (IEI 0xC-, §8.2.4.16).
	MSNetworkFeatureSupport *bool

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain ATTACH REQUEST message. Optional IEs are written in
// the order of Table 8.2.4.1.
// The encoding is appended to b.
func (m *AttachRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAttachRequest)
	w.U8(m.NASKeySetIdentifier.HalfOctet()<<4 | uint8(m.EPSAttachType)&0x07)

	mi, err := m.EPSMobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(mi)

	netCap, err := m.UENetworkCapability.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(netCap)

	w.LVE(m.ESMMessageContainer)

	tv1 := func(iei uint8, set *bool) {
		if set == nil {
			return
		}

		o.TV1(iei, boolBit(*set, 0))
	}

	if m.AdditionalGUTI != nil {
		raw, err := m.AdditionalGUTI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAdditionalGUTI, raw)
	}

	if m.MSNetworkCapability != nil {
		raw, err := m.MSNetworkCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiMSNetworkCapability, raw)
	}

	tv1(ieiTMSIStatus, m.TMSIStatus)

	if m.AdditionalUpdateType != nil {
		o.TV1(ieiAdditionalUpdateType, m.AdditionalUpdateType.Nibble())
	}

	tv1(ieiDeviceProperties, m.DeviceProperties)

	if m.OldGUTIType != nil {
		o.TV1(ieiOldGUTIType, uint8(*m.OldGUTIType)&0x01)
	}

	tv1(ieiMSNetworkFeatureSupport, m.MSNetworkFeatureSupport)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *AttachRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAttachRequest decodes a plain ATTACH REQUEST message.
func ParseAttachRequest(b []byte) (*AttachRequest, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAttachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AttachRequest{EPSAttachType: AttachType(octet & 0x07), NASKeySetIdentifier: nas.ParseKeySetIdentifier(octet >> 4)}

	raw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSMobileIdentity, err = ParseEPSMobileIdentity(raw); err != nil {
		return nil, err
	}

	netCap, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.UENetworkCapability, err = ParseUENetworkCapability(netCap); err != nil {
		return nil, err
	}

	if m.ESMMessageContainer, err = r.LVE(); err != nil {
		return nil, err
	}

	_unrec, err := walkOptionalIEs(r, attachRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiAdditionalGUTI:
			parsed, err := ParseEPSMobileIdentity(value)
			if err != nil {
				return false, err
			}

			m.AdditionalGUTI = &parsed
		case ieiMSNetworkCapability:
			parsed, err := ParseMSNetworkCapability(value)
			if err != nil {
				return false, err
			}

			m.MSNetworkCapability = &parsed
		case ieiTMSIStatus:
			m.TMSIStatus = tv1Flag(value)
		case ieiAdditionalUpdateType:
			if v := tv1Value(value); v != nil {
				aut := ParseAdditionalUpdateType(*v)
				m.AdditionalUpdateType = &aut
			}
		case ieiDeviceProperties:
			m.DeviceProperties = tv1Flag(value)
		case ieiOldGUTIType:
			if v := tv1Value(value); v != nil {
				t := GUTIType(*v & 0x01)
				m.OldGUTIType = &t
			}
		case ieiMSNetworkFeatureSupport:
			m.MSNetworkFeatureSupport = tv1Flag(value)
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// tv1Flag returns bit 1 of the value the walker emits for a single-bit type-1
// IE, or nil if the walker passed no value octet.
func tv1Flag(value []byte) *bool {
	v := tv1Value(value)
	if v == nil {
		return nil
	}

	set := *v&0x01 != 0

	return &set
}

// tv1Value returns the low-nibble value the walker emits for a type-1 IE, or nil
// if the walker passed no value octet.
func tv1Value(value []byte) *uint8 {
	if len(value) == 0 {
		return nil
	}

	v := value[0] & 0x0F

	return &v
}
