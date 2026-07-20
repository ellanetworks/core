// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// IEIs of the ATTACH REQUEST optional information elements (TS 24.301
// Table 8.2.4.1). Full-octet IEIs (< 0x80) are delimited via attachRequestIEs;
// the type-1 IEIs are named by their high nibble and are delimited generically
// by the walker (their value is the low nibble).
const (
	tmsiBasedNRIContainerIEI          = 0x10 // TLV
	oldLocationAreaIDIEI              = 0x13 // TV, 5-octet value
	additionalInformationRequestedIEI = 0x17 // TV, 1-octet value
	oldPTMSISignatureIEI              = 0x19 // TV, 3-octet value
	mobileStationClassmark2IEI        = 0x11 // TLV
	mobileStationClassmark3IEI        = 0x20 // TLV
	msNetworkCapabilityIEI            = 0x31 // TLV
	n1UENetworkCapabilityIEI          = 0x32 // TLV
	ueRadioCapabilityIDAvailIEI       = 0x34 // TLV
	requestedWUSAssistanceIEI         = 0x35 // TLV
	drxParameterNBS1ModeIEI           = 0x36 // TLV
	requestedIMSIOffsetIEI            = 0x38 // TLV
	supportedCodecsIEI                = 0x40 // TLV
	additionalGUTIIEI                 = 0x50 // TLV
	lastVisitedRegisteredTAIIEI       = 0x52 // TV, 5-octet value
	drxParameterIEI                   = 0x5C // TV, 2-octet value
	voiceDomainPreferenceIEI          = 0x5D // TLV
	t3412ExtendedValueIEI             = 0x5E // TLV
	t3324ValueIEI                     = 0x6A // TLV
	ueStatusIEI                       = 0x6D // TLV
	extendedDRXParametersIEI          = 0x6E // TLV
	ueAdditionalSecurityCapIEI        = 0x6F // TLV

	// Type-1 (TV, ½ octet) IEs: the IEI is the high nibble, the value the low
	// nibble (TS 24.301).
	tmsiStatusIEI              = 0x90 // 9-
	msNetworkFeatureSupportIEI = 0xC0 // C-
	devicePropertiesIEI        = 0xD0 // D-
	oldGUTITypeIEI             = 0xE0 // E-
	additionalUpdateTypeIEI    = 0xF0 // F-
)

// attachRequestIEs are the full-octet optional IEs of the ATTACH REQUEST, in the
// order TS 24.301 Table 8.2.4.1 lists them, so the walker delimits every one and
// no IE hides those after it. Type-1 IEs (IEI ≥ 0x80) are delimited generically
// and are not listed.
var attachRequestIEs = []common.OptionalIE{
	{IEI: oldPTMSISignatureIEI, Format: common.IETV3, Len: 3},
	{IEI: additionalGUTIIEI, Format: common.IETLV},
	{IEI: lastVisitedRegisteredTAIIEI, Format: common.IETV3, Len: 5},
	{IEI: drxParameterIEI, Format: common.IETV3, Len: 2},
	{IEI: msNetworkCapabilityIEI, Format: common.IETLV},
	{IEI: oldLocationAreaIDIEI, Format: common.IETV3, Len: 5},
	{IEI: mobileStationClassmark2IEI, Format: common.IETLV},
	{IEI: mobileStationClassmark3IEI, Format: common.IETLV},
	{IEI: supportedCodecsIEI, Format: common.IETLV},
	{IEI: voiceDomainPreferenceIEI, Format: common.IETLV},
	{IEI: tmsiBasedNRIContainerIEI, Format: common.IETLV},
	{IEI: t3324ValueIEI, Format: common.IETLV},
	{IEI: t3412ExtendedValueIEI, Format: common.IETLV},
	{IEI: extendedDRXParametersIEI, Format: common.IETLV},
	{IEI: ueAdditionalSecurityCapIEI, Format: common.IETLV},
	{IEI: ueStatusIEI, Format: common.IETLV},
	{IEI: additionalInformationRequestedIEI, Format: common.IETV3, Len: 1},
	{IEI: n1UENetworkCapabilityIEI, Format: common.IETLV},
	{IEI: ueRadioCapabilityIDAvailIEI, Format: common.IETLV},
	{IEI: requestedWUSAssistanceIEI, Format: common.IETLV},
	{IEI: drxParameterNBS1ModeIEI, Format: common.IETLV},
	{IEI: requestedIMSIOffsetIEI, Format: common.IETLV},
}

// AttachRequest is the ATTACH REQUEST message (TS 24.301 §8.2.4). The optional
// IEs are carried as their raw values, not decoded further: a nil slice (or nil
// pointer, for the type-1 half-octet IEs) means the UE omitted the IE. The values
// round-trip byte-for-byte through Marshal.
type AttachRequest struct {
	EPSAttachType       uint8
	NASKeySetIdentifier uint8
	EPSMobileIdentity   EPSMobileIdentity
	UENetworkCapability []byte
	ESMMessageContainer []byte

	OldPTMSISignature               []byte // IEI 0x19, §8.2.4.2
	AdditionalGUTI                  []byte // IEI 0x50, §8.2.4.3
	LastVisitedRegisteredTAI        []byte // IEI 0x52, §8.2.4.4
	DRXParameter                    []byte // IEI 0x5C, §8.2.4.5
	MSNetworkCapability             []byte // IEI 0x31, §8.2.4.6
	OldLocationAreaID               []byte // IEI 0x13, §8.2.4.7
	TMSIStatus                      *uint8 // IEI 0x9-, §8.2.4.8
	MobileStationClassmark2         []byte // IEI 0x11, §8.2.4.9
	MobileStationClassmark3         []byte // IEI 0x20, §8.2.4.10
	SupportedCodecs                 []byte // IEI 0x40, §8.2.4.11
	AdditionalUpdateType            *uint8 // IEI 0xF-, §8.2.4.12
	VoiceDomainPreference           []byte // IEI 0x5D, §8.2.4.13
	DeviceProperties                *uint8 // IEI 0xD-, §8.2.4.14
	OldGUTIType                     *uint8 // IEI 0xE-, §8.2.4.15
	MSNetworkFeatureSupport         *uint8 // IEI 0xC-, §8.2.4.16
	TMSIBasedNRIContainer           []byte // IEI 0x10, §8.2.4.17
	T3324Value                      []byte // IEI 0x6A, §8.2.4.18
	T3412ExtendedValue              []byte // IEI 0x5E, §8.2.4.19
	ExtendedDRXParameters           []byte // IEI 0x6E, §8.2.4.20
	UEAdditionalSecurityCapability  []byte // IEI 0x6F, §8.2.4.21
	UEStatus                        []byte // IEI 0x6D, §8.2.4.22
	AdditionalInformationRequested  []byte // IEI 0x17, §8.2.4.23
	N1UENetworkCapability           []byte // IEI 0x32, §8.2.4.24
	UERadioCapabilityIDAvailability []byte // IEI 0x34, §8.2.4.25
	RequestedWUSAssistance          []byte // IEI 0x35, §8.2.4.26
	DRXParameterNBS1Mode            []byte // IEI 0x36, §8.2.4.27
	RequestedIMSIOffset             []byte // IEI 0x38, §8.2.4.28
}

// Marshal encodes the plain ATTACH REQUEST message. Optional IEs are written in
// the order of Table 8.2.4.1.
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

	var marshalErr error

	tlv := func(iei uint8, v []byte) {
		if marshalErr != nil || len(v) == 0 {
			return
		}

		w.U8(iei)

		if err := w.LV(v); err != nil {
			marshalErr = err
		}
	}

	tv := func(iei uint8, v []byte) {
		if marshalErr != nil || len(v) == 0 {
			return
		}

		w.U8(iei)
		w.Raw(v)
	}

	tv1 := func(iei uint8, v *uint8) {
		if marshalErr != nil || v == nil {
			return
		}

		w.U8(iei | (*v & 0x0F))
	}

	tv(oldPTMSISignatureIEI, m.OldPTMSISignature)
	tlv(additionalGUTIIEI, m.AdditionalGUTI)
	tv(lastVisitedRegisteredTAIIEI, m.LastVisitedRegisteredTAI)
	tv(drxParameterIEI, m.DRXParameter)
	tlv(msNetworkCapabilityIEI, m.MSNetworkCapability)
	tv(oldLocationAreaIDIEI, m.OldLocationAreaID)
	tv1(tmsiStatusIEI, m.TMSIStatus)
	tlv(mobileStationClassmark2IEI, m.MobileStationClassmark2)
	tlv(mobileStationClassmark3IEI, m.MobileStationClassmark3)
	tlv(supportedCodecsIEI, m.SupportedCodecs)
	tv1(additionalUpdateTypeIEI, m.AdditionalUpdateType)
	tlv(voiceDomainPreferenceIEI, m.VoiceDomainPreference)
	tv1(devicePropertiesIEI, m.DeviceProperties)
	tv1(oldGUTITypeIEI, m.OldGUTIType)
	tv1(msNetworkFeatureSupportIEI, m.MSNetworkFeatureSupport)
	tlv(tmsiBasedNRIContainerIEI, m.TMSIBasedNRIContainer)
	tlv(t3324ValueIEI, m.T3324Value)
	tlv(t3412ExtendedValueIEI, m.T3412ExtendedValue)
	tlv(extendedDRXParametersIEI, m.ExtendedDRXParameters)
	tlv(ueAdditionalSecurityCapIEI, m.UEAdditionalSecurityCapability)
	tlv(ueStatusIEI, m.UEStatus)
	tv(additionalInformationRequestedIEI, m.AdditionalInformationRequested)
	tlv(n1UENetworkCapabilityIEI, m.N1UENetworkCapability)
	tlv(ueRadioCapabilityIDAvailIEI, m.UERadioCapabilityIDAvailability)
	tlv(requestedWUSAssistanceIEI, m.RequestedWUSAssistance)
	tlv(drxParameterNBS1ModeIEI, m.DRXParameterNBS1Mode)
	tlv(requestedIMSIOffsetIEI, m.RequestedIMSIOffset)

	if marshalErr != nil {
		return nil, marshalErr
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
		switch iei {
		case oldPTMSISignatureIEI:
			m.OldPTMSISignature = value
		case additionalGUTIIEI:
			m.AdditionalGUTI = value
		case lastVisitedRegisteredTAIIEI:
			m.LastVisitedRegisteredTAI = value
		case drxParameterIEI:
			m.DRXParameter = value
		case msNetworkCapabilityIEI:
			m.MSNetworkCapability = value
		case oldLocationAreaIDIEI:
			m.OldLocationAreaID = value
		case mobileStationClassmark2IEI:
			m.MobileStationClassmark2 = value
		case mobileStationClassmark3IEI:
			m.MobileStationClassmark3 = value
		case supportedCodecsIEI:
			m.SupportedCodecs = value
		case voiceDomainPreferenceIEI:
			m.VoiceDomainPreference = value
		case tmsiBasedNRIContainerIEI:
			m.TMSIBasedNRIContainer = value
		case t3324ValueIEI:
			m.T3324Value = value
		case t3412ExtendedValueIEI:
			m.T3412ExtendedValue = value
		case extendedDRXParametersIEI:
			m.ExtendedDRXParameters = value
		case ueAdditionalSecurityCapIEI:
			m.UEAdditionalSecurityCapability = value
		case ueStatusIEI:
			m.UEStatus = value
		case additionalInformationRequestedIEI:
			m.AdditionalInformationRequested = value
		case n1UENetworkCapabilityIEI:
			m.N1UENetworkCapability = value
		case ueRadioCapabilityIDAvailIEI:
			m.UERadioCapabilityIDAvailability = value
		case requestedWUSAssistanceIEI:
			m.RequestedWUSAssistance = value
		case drxParameterNBS1ModeIEI:
			m.DRXParameterNBS1Mode = value
		case requestedIMSIOffsetIEI:
			m.RequestedIMSIOffset = value
		case tmsiStatusIEI:
			m.TMSIStatus = tv1Value(value)
		case additionalUpdateTypeIEI:
			m.AdditionalUpdateType = tv1Value(value)
		case devicePropertiesIEI:
			m.DeviceProperties = tv1Value(value)
		case oldGUTITypeIEI:
			m.OldGUTIType = tv1Value(value)
		case msNetworkFeatureSupportIEI:
			m.MSNetworkFeatureSupport = tv1Value(value)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
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
