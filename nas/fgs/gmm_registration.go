// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// 5GS registration type values (TS 24.501 §9.11.3.7, table 9.11.3.7.1).
const (
	RegistrationTypeInitial          uint8 = 0x01
	RegistrationTypeMobilityUpdating uint8 = 0x02
	RegistrationTypePeriodicUpdating uint8 = 0x03
	RegistrationTypeEmergency        uint8 = 0x04
	RegistrationTypeReserved         uint8 = 0x07
)

// 5GS DRX parameter values (TS 24.501 §9.11.3.2A, table 9.11.3.2A.1).
const (
	DRXValueNotSpecified  uint8 = 0x00
	DRXCycleParameterT32  uint8 = 0x01
	DRXCycleParameterT64  uint8 = 0x02
	DRXCycleParameterT128 uint8 = 0x03
	DRXCycleParameterT256 uint8 = 0x04
)

// NgKSINoKeyAvailable is the ngKSI value indicating no 5G NAS security context is
// available (TS 24.501 §9.11.3.32).
const NgKSINoKeyAvailable uint8 = 0x07

// RegistrationRequest is the REGISTRATION REQUEST message (TS 24.501 §8.2.6). Ella
// reads the fields below; the many other optional IEs are still walked (so a later
// IE Ella reads is reached) but their values are discarded.
type RegistrationRequest struct {
	RegistrationType uint8  // bits 1-3 of the ngKSI/registration-type octet
	FOR              uint8  // follow-on request pending, bit 4
	NgKSI            uint8  // NAS key set identifier, bits 5-7
	TSC              uint8  // type of security context flag, bit 8
	MobileIdentity   []byte // mandatory 5GS mobile identity (type 6, LVE)

	Capability5GMM          []byte // IEI 0x10
	UESecurityCapability    []byte // IEI 0x2E
	UplinkDataStatus        []byte // IEI 0x40
	PDUSessionStatus        []byte // IEI 0x50
	AllowedPDUSessionStatus []byte // IEI 0x25
	RequestedDRXParameters  []byte // IEI 0x51
	NASMessageContainer     []byte // IEI 0x71
	MICOIndication          *uint8 // IEI 0xB0 (type 1)
	UpdateType5GS           *uint8 // IEI 0x53 (5GS update type value)
}

// registrationRequestIEs is the full-octet optional-IE table of the REGISTRATION
// REQUEST (TS 24.501 §8.2.6, table 8.2.6.1.1). Type-1 IEs (0xC- non-current native
// NAS KSI, 0xB- MICO indication, 0x9- network slicing indication) are delimited
// generically by the walker; this table lets it step over the full-octet optional
// IEs so a later IE Ella reads is still reached.
var registrationRequestIEs = []common.OptionalIE{
	{IEI: 0x10, Format: common.IETLV},         // 5GMM capability
	{IEI: 0x17, Format: common.IETLV},         // S1 UE network capability
	{IEI: 0x18, Format: common.IETLV},         // UE's usage setting
	{IEI: 0x25, Format: common.IETLV},         // allowed PDU session status
	{IEI: 0x2B, Format: common.IETLV},         // UE status
	{IEI: 0x2E, Format: common.IETLV},         // UE security capability
	{IEI: 0x2F, Format: common.IETLV},         // requested NSSAI
	{IEI: 0x40, Format: common.IETLV},         // uplink data status
	{IEI: 0x50, Format: common.IETLV},         // PDU session status
	{IEI: 0x51, Format: common.IETLV},         // requested DRX parameters
	{IEI: 0x52, Format: common.IETV3, Len: 6}, // last visited registered TAI
	{IEI: 0x53, Format: common.IETLV},         // 5GS update type
	{IEI: 0x60, Format: common.IETLV},         // EPS bearer context status
	{IEI: 0x70, Format: common.IETLVE},        // EPS NAS message container
	{IEI: 0x71, Format: common.IETLVE},        // NAS message container
	{IEI: 0x74, Format: common.IETLVE},        // LADN indication
	{IEI: 0x77, Format: common.IETLVE},        // additional GUTI
	{IEI: 0x7B, Format: common.IETLVE},        // payload container
}

// ParseRegistrationRequest decodes a REGISTRATION REQUEST message.
func ParseRegistrationRequest(b []byte) (*RegistrationRequest, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgRegistrationRequest); err != nil {
		return nil, err
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	mi, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &RegistrationRequest{
		RegistrationType: octet1 & 0x07,
		FOR:              octet1 >> 3 & 0x01,
		NgKSI:            octet1 >> 4 & 0x07,
		TSC:              octet1 >> 7 & 0x01,
		MobileIdentity:   mi,
	}

	if _, err := common.WalkOptionalIEs(r, registrationRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case 0x10:
			out.Capability5GMM = value
		case 0x2E:
			out.UESecurityCapability = value
		case 0x40:
			out.UplinkDataStatus = value
		case 0x50:
			out.PDUSessionStatus = value
		case 0x25:
			out.AllowedPDUSessionStatus = value
		case 0x51:
			out.RequestedDRXParameters = value
		case 0x71:
			out.NASMessageContainer = value
		case 0x53:
			if len(value) > 0 {
				v := value[0]
				out.UpdateType5GS = &v
			}
		case 0xB0: // MICO indication (type 1): value is the low nibble
			if len(value) > 0 {
				v := value[0]
				out.MICOIndication = &v
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// RAAI returns the registration area allocation indication of the MICO indication
// IE, or 0 when absent (TS 24.501 §9.11.3.31).
func (m *RegistrationRequest) RAAI() uint8 {
	if m.MICOIndication == nil {
		return 0
	}

	return *m.MICOIndication & 0x01
}

// NGRanRcu returns the NG-RAN radio capability update indication of the 5GS update
// type IE, or 0 when absent (TS 24.501 §9.11.3.9A).
func (m *RegistrationRequest) NGRanRcu() uint8 {
	if m.UpdateType5GS == nil {
		return 0
	}

	return *m.UpdateType5GS >> 1 & 0x01
}

// DRXValue returns the requested DRX parameter value, or 0 when absent
// (TS 24.501 §9.11.3.2A).
func (m *RegistrationRequest) DRXValue() uint8 {
	if len(m.RequestedDRXParameters) == 0 {
		return 0
	}

	return m.RequestedDRXParameters[0] & 0x0F
}

// RegistrationAccept is the REGISTRATION ACCEPT message (TS 24.501 §8.2.7). The
// mandatory 5GS registration result is followed by optional IEs, several of them
// (GUTI, equivalent PLMNs, TAI list, allowed NSSAI, network feature support)
// supplied as their already-encoded IE value.
type RegistrationAccept struct {
	RegistrationResult           uint8  // 5GS registration result value (bits 1-3)
	GUTI                         []byte // optional (IEI 0x77): 5GS mobile identity value
	EquivalentPlmns              []byte // optional (IEI 0x4A)
	TAIList                      []byte // optional (IEI 0x54)
	AllowedNSSAI                 []byte // optional (IEI 0x15)
	NetworkFeatureSupport        []byte // optional (IEI 0x21)
	PDUSessionStatus             []byte // optional (IEI 0x50)
	PDUSessionReactivationResult []byte // optional (IEI 0x26)
	ReactivationResultErrorCause []byte // optional (IEI 0x72), TLV-E
	T3512Value                   *uint8 // optional (IEI 0x5E), GPRS timer 3 octet
	NegotiatedDRX                *uint8 // optional (IEI 0x51), DRX value
}

// Marshal encodes the plain REGISTRATION ACCEPT message.
func (m *RegistrationAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgRegistrationAccept)

	// 5GS registration result (mandatory, LV, 1-octet value).
	if err := w.LV([]byte{m.RegistrationResult & 0x07}); err != nil {
		return nil, err
	}

	if m.GUTI != nil {
		writeTLVE(&w, ieiGUTI5G, m.GUTI)
	}

	if m.EquivalentPlmns != nil {
		writeTLV(&w, ieiEquivalentPlmns, m.EquivalentPlmns)
	}

	if m.TAIList != nil {
		writeTLV(&w, ieiTAIList, m.TAIList)
	}

	if m.AllowedNSSAI != nil {
		writeTLV(&w, ieiAllowedNSSAI, m.AllowedNSSAI)
	}

	if m.NetworkFeatureSupport != nil {
		writeTLV(&w, ieiNetworkFeature, m.NetworkFeatureSupport)
	}

	if m.PDUSessionStatus != nil {
		writeTLV(&w, ieiPDUSessionStatus, m.PDUSessionStatus)
	}

	if m.PDUSessionReactivationResult != nil {
		writeTLV(&w, ieiPDUReactResult, m.PDUSessionReactivationResult)
	}

	if m.ReactivationResultErrorCause != nil {
		writeTLVE(&w, ieiPDUReactErrCause, m.ReactivationResultErrorCause)
	}

	if m.T3512Value != nil {
		writeTLV(&w, ieiT3512Value, []byte{*m.T3512Value})
	}

	if m.NegotiatedDRX != nil {
		writeTLV(&w, ieiNegotiatedDRX, []byte{*m.NegotiatedDRX})
	}

	return w.Bytes(), nil
}
