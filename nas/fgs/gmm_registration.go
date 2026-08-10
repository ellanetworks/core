// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// 5GS registration type values (TS 24.501 §9.11.3.7, table 9.11.3.7.1).
const (
	RegistrationTypeInitial                         RegistrationType = 0x01
	RegistrationTypeMobilityUpdating                RegistrationType = 0x02
	RegistrationTypePeriodicUpdating                RegistrationType = 0x03
	RegistrationTypeEmergency                       RegistrationType = 0x04
	RegistrationTypeSNPNOnboarding                  RegistrationType = 0x05
	RegistrationTypeDisasterRoamingMobilityUpdating RegistrationType = 0x06
	RegistrationTypeDisasterRoamingInitial          RegistrationType = 0x07
)

// 5GS DRX parameter values (TS 24.501 §9.11.3.2A, table 9.11.3.2A.1).
const (
	DRXValueNotSpecified  DRXValue = 0x00
	DRXCycleParameterT32  DRXValue = 0x01
	DRXCycleParameterT64  DRXValue = 0x02
	DRXCycleParameterT128 DRXValue = 0x03
	DRXCycleParameterT256 DRXValue = 0x04
)

// RegistrationRequest is the REGISTRATION REQUEST message (TS 24.501 §8.2.6). Ella
// reads the fields below; the many other optional IEs are still walked (so a later
// IE Ella reads is reached) but their values are discarded.
type RegistrationRequest struct {
	RegistrationType RegistrationType     // bits 1-3 of the ngKSI/registration-type octet
	FOR              bool                 // follow-on request pending, bit 4
	NgKSI            nas.KeySetIdentifier // bits 5-8
	MobileIdentity   MobileIdentity       // mandatory 5GS mobile identity (type 6, LVE)

	GMMCapability        *GMMCapability        // IEI 0x10
	UESecurityCapability *UESecurityCapability // IEI 0x2E

	// Opaque for the same reason SECURITY MODE COMMAND's replayed copy is: the UE
	// compares the replay byte-for-byte (TS 33.501 §6.7.2), so decoding and
	// re-encoding could only lose that. Modelled rather than left to Unrecognized
	// because Critical is only consulted when the parse callback rejects a value,
	// so an element with no case there can never fire it.
	S1UENetworkCapability []byte // IEI 0x17

	RequestedNSSAI          NSSAI           // IEI 0x2F
	UplinkDataStatus        *PSIBitmap      // IEI 0x40
	PDUSessionStatus        *PSIBitmap      // IEI 0x50
	AllowedPDUSessionStatus *PSIBitmap      // IEI 0x25
	UEStatus                *UEStatus       // IEI 0x2B
	RequestedDRXParameters  *DRXParameter   // IEI 0x51
	NASMessageContainer     []byte          // IEI 0x71
	MICOIndication          *MICOIndication // IEI 0xB0 (type 1)
	UpdateType5GS           *UpdateType5GS  // IEI 0x53

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// registrationRequestIEs is the full-octet optional-IE table of the REGISTRATION
// REQUEST (TS 24.501 §8.2.6, table 8.2.6.1.1). Type-1 IEs (0xC- non-current native
// NAS KSI, 0xB- MICO indication, 0x9- network slicing indication) are delimited
// generically by the walker; this table lets it step over the full-octet optional
// IEs so a later IE Ella reads is still reached.
var registrationRequestIEs = []nas.OptionalIE{
	{IEI: ieiGMMCapability, Format: nas.IETLV, Name: "5GMM capability"},
	{IEI: ieiS1UENetworkCapability, Format: nas.IETLV, Critical: true, Name: "S1 UE network capability"},
	{IEI: ieiUEUsageSetting, Format: nas.IETLV, Name: "UE usage setting"},
	{IEI: ieiAllowedPDUSessionStatus, Format: nas.IETLV, Name: "Allowed PDU session status"},
	{IEI: ieiUEStatus, Format: nas.IETLV, Name: "UE status"},
	{IEI: ieiUESecurityCapability, Format: nas.IETLV, Critical: true, Name: "UE security capability"},
	{IEI: ieiRequestedNSSAI, Format: nas.IETLV, Name: "Requested NSSAI"},
	{IEI: ieiUplinkDataStatus, Format: nas.IETLV, Name: "Uplink data status"},
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
	{IEI: ieiRequestedDRXParameters, Format: nas.IETLV, Name: "Requested DRX parameters"},
	{IEI: ieiLastVisitedTAI, Format: nas.IETV3, Len: 6, Name: "Last visited TAI"},
	{IEI: ieiUpdateType5GS, Format: nas.IETLV, Name: "5GS update type"},
	{IEI: ieiEPSBearerContextStatus, Format: nas.IETLV, Name: "EPS bearer context status"},
	{IEI: ieiEPSNASMessageContainer, Format: nas.IETLVE, Name: "EPS NAS message container"},
	{IEI: ieiNASMessageContainer, Format: nas.IETLVE, Critical: true, Name: "NAS message container"},
	{IEI: ieiLADNIndication, Format: nas.IETLVE, Name: "LADN indication"},
	{IEI: ieiGUTI5G, Format: nas.IETLVE, Name: "5G-GUTI"}, // additional GUTI
	{IEI: ieiPayloadContainer, Format: nas.IETLVE, Name: "Payload container"},
}

// AppendBinary encodes the plain REGISTRATION REQUEST message. The optional
// elements are emitted in the order TS 24.501 table 8.2.6.1.1 lists them.
// The encoding is appended to b.
func (m *RegistrationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgRegistrationRequest)
	w.U8(registrationHeader{RegistrationType: m.RegistrationType, FOR: m.FOR, NgKSI: m.NgKSI}.octet())

	mi, err := m.MobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LVE(mi)

	if m.GMMCapability != nil {
		raw, err := m.GMMCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiGMMCapability, raw)
	}

	if m.UESecurityCapability != nil {
		raw, err := m.UESecurityCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiUESecurityCapability, raw)
	}

	if m.RequestedNSSAI != nil {
		raw, err := m.RequestedNSSAI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiRequestedNSSAI, raw)
	}

	// Table 8.2.6.1.1 orders 2E, 2F, 52 then 17, so a decoded message re-encodes
	// with its elements in the order the table lists.
	if m.S1UENetworkCapability != nil {
		o.TLV(ieiS1UENetworkCapability, m.S1UENetworkCapability)
	}

	if m.UplinkDataStatus != nil {
		raw, err := m.UplinkDataStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiUplinkDataStatus, raw)
	}

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	if m.MICOIndication != nil {
		o.TV1(ieiMICOIndication, m.MICOIndication.Nibble())
	}

	if m.UEStatus != nil {
		o.TLV(ieiUEStatus, m.UEStatus.MarshalBinary())
	}

	if m.AllowedPDUSessionStatus != nil {
		raw, err := m.AllowedPDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAllowedPDUSessionStatus, raw)
	}

	if m.RequestedDRXParameters != nil {
		raw, err := m.RequestedDRXParameters.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiRequestedDRXParameters, raw)
	}

	if m.UpdateType5GS != nil {
		raw, err := m.UpdateType5GS.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiUpdateType5GS, raw)
	}

	if m.NASMessageContainer != nil {
		o.TLVE(ieiNASMessageContainer, m.NASMessageContainer)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *RegistrationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// registrationHeader holds the fields packed into the ngKSI/registration-type
// octet of the REGISTRATION REQUEST (TS 24.501 §9.11.3.7 and §9.11.3.32).
type registrationHeader struct {
	RegistrationType RegistrationType
	FOR              bool
	NgKSI            nas.KeySetIdentifier
}

func parseRegistrationHeader(v uint8) registrationHeader {
	return registrationHeader{
		RegistrationType: RegistrationType(v & 0x07),
		FOR:              v&0x08 != 0,
		NgKSI:            nas.ParseKeySetIdentifier(v >> 4),
	}
}

func (h registrationHeader) octet() uint8 {
	o := uint8(h.RegistrationType)&0x07 | h.NgKSI.HalfOctet()<<4
	if h.FOR {
		o |= 0x08
	}

	return o
}

// ParseRegistrationRequest decodes the message.
func ParseRegistrationRequest(b []byte) (*RegistrationRequest, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgRegistrationRequest); err != nil {
		return nil, err
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	raw, err := r.LVE()
	if err != nil {
		return nil, err
	}

	mi, err := ParseMobileIdentity(raw)
	if err != nil {
		return nil, err
	}

	hdr := parseRegistrationHeader(octet1)

	out := &RegistrationRequest{
		RegistrationType: hdr.RegistrationType,
		FOR:              hdr.FOR,
		NgKSI:            hdr.NgKSI,
		MobileIdentity:   mi,
	}

	_unrec, err := walkOptionalIEs(r, registrationRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiGMMCapability:
			parsed, err := ParseGMMCapability(value)
			if err != nil {
				return false, err
			}

			out.GMMCapability = &parsed
		case ieiUESecurityCapability:
			parsed, err := ParseUESecurityCapability(value)
			if err != nil {
				return false, err
			}

			out.UESecurityCapability = &parsed
		case ieiS1UENetworkCapability:
			// TS 24.301 §9.9.3.34 bounds the value at 2..13 octets. Length is all
			// that can be checked without decoding, and decoding is what must not
			// happen here — but it is enough to make Critical fire.
			if len(value) < 2 || len(value) > 13 {
				return false, fmt.Errorf("nas/fgs: S1 UE network capability is %d octets, want 2..13", len(value))
			}

			out.S1UENetworkCapability = value
		case ieiRequestedNSSAI:
			parsed, err := ParseNSSAI(value)
			if err != nil {
				return false, err
			}

			out.RequestedNSSAI = parsed
		case ieiUplinkDataStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.UplinkDataStatus = &bitmap
		case ieiPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionStatus = &bitmap
		case ieiUEStatus:
			parsed, err := ParseUEStatus(value)
			if err != nil {
				return false, err
			}

			out.UEStatus = &parsed
		case ieiAllowedPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.AllowedPDUSessionStatus = &bitmap
		case ieiRequestedDRXParameters:
			drx, err := ParseDRXParameter(value)
			if err != nil {
				return false, err
			}

			out.RequestedDRXParameters = &drx
		case ieiNASMessageContainer:
			out.NASMessageContainer = value
		case ieiUpdateType5GS:
			ut, err := ParseUpdateType5GS(value)
			if err != nil {
				return false, err
			}

			out.UpdateType5GS = &ut
		case ieiMICOIndication: // type 1: the value is the low nibble
			if len(value) != 1 {
				return false, fmt.Errorf("nas/fgs: MICO indication is %d octets, want 1", len(value))
			}

			mico := ParseMICOIndication(value[0])
			out.MICOIndication = &mico
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// RegistrationAccept is the REGISTRATION ACCEPT message (TS 24.501 §8.2.7). The
// mandatory 5GS registration result is followed by optional elements, emitted in
// the order TS 24.501 table 8.2.7.1.1 lists them.
type RegistrationAccept struct {
	RegistrationResult           RegistrationResult     // 5GS registration result value (bits 1-3)
	RegistrationResultRest       []byte                 // octets beyond the first, present from Rel-16
	GUTI                         *MobileIdentity        // optional (IEI 0x77): 5G-GUTI
	TAIList                      *TAIList               // optional (IEI 0x54)
	AllowedNSSAI                 NSSAI                  // optional (IEI 0x15)
	NetworkFeatureSupport        *NetworkFeatureSupport // optional (IEI 0x21)
	PDUSessionStatus             *PSIBitmap             // optional (IEI 0x50)
	PDUSessionReactivationResult *PSIBitmap             // optional (IEI 0x26)
	T3512                        *nas.GPRSTimer3        // optional (IEI 0x5E), GPRS timer 3 octet
	NegotiatedDRX                *DRXParameter          // optional (IEI 0x51)

	EquivalentPLMNs              nas.PLMNList                 // optional (IEI 0x4A)
	ReactivationResultErrorCause ReactivationResultErrorCause // optional (IEI 0x72), TLV-E
	ConfiguredNSSAI              NSSAI                        // optional (IEI 0x31)
	Non3GppDeregistrationTimer   *nas.GPRSTimer2              // optional (IEI 0x5D), a GPRS timer 2
	T3502                        *nas.GPRSTimer2              // optional (IEI 0x16), a GPRS timer 2
	MICOIndication               *MICOIndication              // optional (IEI 0xB0), type 1

	// Opaque containers: the library carries them without interpreting them.
	SORTransparentContainer []byte                      // optional (IEI 0x73), TLV-E
	EAP                     []byte                      // optional (IEI 0x78), TLV-E
	EPSBearerContextStatus  *nas.EPSBearerContextStatus // optional (IEI 0x60)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var registrationAcceptIEs = []nas.OptionalIE{
	{IEI: ieiGUTI5G, Format: nas.IETLVE, Name: "5G-GUTI"},
	{IEI: ieiRejectedNSSAI, Format: nas.IETLV, Name: "Rejected NSSAI"},
	{IEI: ieiEquivalentPlmns, Format: nas.IETLV, Name: "Equivalent PLMNs"},
	{IEI: ieiTAIList, Format: nas.IETLV, Name: "TAI list"},
	{IEI: ieiAllowedNSSAI, Format: nas.IETLV, Name: "Allowed NSSAI"},
	{IEI: ieiConfiguredNSSAI, Format: nas.IETLV, Name: "Configured NSSAI"},
	{IEI: ieiT3502Value, Format: nas.IETLV, Name: "T3502 value"},
	{IEI: ieiNetworkFeature, Format: nas.IETLV, Name: "5GS network feature support"},
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
	{IEI: ieiPDUReactResult, Format: nas.IETLV, Name: "PDU session reactivation result"},
	{IEI: ieiPDUReactErrCause, Format: nas.IETLVE, Name: "PDU session reactivation result error cause"},
	{IEI: ieiLADNInformation, Format: nas.IETLVE, Name: "LADN information"},
	{IEI: ieiServiceAreaList, Format: nas.IETLV, Name: "Service area list"},
	{IEI: ieiT3512Value, Format: nas.IETLV, Name: "T3512 value"},
	{IEI: ieiNon3GppDeregTimer, Format: nas.IETLV, Name: "Non-3GPP de-registration timer"},
	{IEI: ieiEmergencyNumberList, Format: nas.IETLV, Name: "Emergency number list"},
	{IEI: ieiExtEmergencyNumberList, Format: nas.IETLVE, Name: "Extended emergency number list"},
	{IEI: ieiSORContainer, Format: nas.IETLVE, Name: "SOR container"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
	{IEI: ieiOperatorAccessCategory, Format: nas.IETLVE, Name: "Operator access category"},
	{IEI: ieiNegotiatedDRX, Format: nas.IETLV, Name: "Negotiated DRX parameters"},
	{IEI: ieiEPSBearerContextStatus, Format: nas.IETLV, Name: "EPS bearer context status"},
}

// ParseRegistrationAccept decodes the message.
func ParseRegistrationAccept(b []byte) (*RegistrationAccept, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgRegistrationAccept); err != nil {
		return nil, err
	}

	result, err := r.LV()
	if err != nil {
		return nil, err
	}

	if len(result) < 1 {
		return nil, fmt.Errorf("nas/fgs: registration accept: empty 5GS registration result")
	}

	out := &RegistrationAccept{RegistrationResult: RegistrationResult(result[0])}

	if len(result) > 1 {
		out.RegistrationResultRest = result[1:]
	}

	_unrec, err := walkOptionalIEs(r, registrationAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiGUTI5G:
			parsed, err := ParseMobileIdentity(value)
			if err != nil {
				return false, err
			}

			out.GUTI = &parsed
		case ieiEquivalentPlmns:
			list, err := nas.ParsePLMNList(value)
			if err != nil {
				return false, err
			}

			out.EquivalentPLMNs = list
		case ieiPDUReactErrCause:
			causes, err := ParseReactivationResultErrorCause(value)
			if err != nil {
				return false, err
			}

			out.ReactivationResultErrorCause = causes
		case ieiTAIList:
			parsed, err := ParseTAIList(value)
			if err != nil {
				return false, err
			}

			out.TAIList = &parsed
		case ieiAllowedNSSAI:
			parsed, err := ParseNSSAI(value)
			if err != nil {
				return false, err
			}

			out.AllowedNSSAI = parsed
		case ieiConfiguredNSSAI:
			parsed, err := ParseNSSAI(value)
			if err != nil {
				return false, err
			}

			out.ConfiguredNSSAI = parsed
		case ieiT3502Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3502 = &timer
		case ieiNetworkFeature:
			parsed, err := ParseNetworkFeatureSupport(value)
			if err != nil {
				return false, err
			}

			out.NetworkFeatureSupport = &parsed
		case ieiPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionStatus = &bitmap
		case ieiPDUReactResult:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionReactivationResult = &bitmap
		case ieiT3512Value:
			if len(value) == 0 {
				return false, nil
			}

			timer, err := nas.ParseGPRSTimer3(value)
			if err != nil {
				return false, err
			}

			out.T3512 = &timer
		case ieiNon3GppDeregTimer:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.Non3GppDeregistrationTimer = &timer
		case ieiSORContainer:
			out.SORTransparentContainer = value
		case ieiEAPMessage:
			out.EAP = value
		case ieiNegotiatedDRX:
			drx, err := ParseDRXParameter(value)
			if err != nil {
				return false, err
			}

			out.NegotiatedDRX = &drx
		case ieiEPSBearerContextStatus:
			status, err := nas.ParseEPSBearerContextStatus(value)
			if err != nil {
				return false, err
			}

			out.EPSBearerContextStatus = &status
		case ieiMICOIndication: // type 1: the value is the low nibble
			if len(value) != 1 {
				return false, fmt.Errorf("nas/fgs: MICO indication is %d octets, want 1", len(value))
			}

			mico := ParseMICOIndication(value[0])
			out.MICOIndication = &mico
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain REGISTRATION ACCEPT message.
// The encoding is appended to b.
func (m *RegistrationAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgRegistrationAccept)

	// 5GS registration result (mandatory, LV, 1-octet value): result value
	// (bits 1-3), SMS-allowed (bit 4), NSSAA (bit 5), emergency-registered
	// (bit 6) — TS 24.501 §9.11.3.6.
	w.LVFunc(func(c *nas.Writer) {
		c.U8(uint8(m.RegistrationResult))
		c.Raw(m.RegistrationResultRest)
	})

	if m.GUTI != nil {
		raw, err := m.GUTI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiGUTI5G, raw)
	}

	if m.EquivalentPLMNs != nil {
		raw, err := m.EquivalentPLMNs.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiEquivalentPlmns, raw)
	}

	if m.TAIList != nil {
		raw, err := m.TAIList.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiTAIList, raw)
	}

	if m.AllowedNSSAI != nil {
		raw, err := m.AllowedNSSAI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAllowedNSSAI, raw)
	}

	if m.ConfiguredNSSAI != nil {
		raw, err := m.ConfiguredNSSAI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiConfiguredNSSAI, raw)
	}

	if m.NetworkFeatureSupport != nil {
		raw, err := m.NetworkFeatureSupport.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNetworkFeature, raw)
	}

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	if m.PDUSessionReactivationResult != nil {
		raw, err := m.PDUSessionReactivationResult.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUReactResult, raw)
	}

	if m.ReactivationResultErrorCause != nil {
		raw, err := m.ReactivationResultErrorCause.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiPDUReactErrCause, raw)
	}

	if m.MICOIndication != nil {
		o.TV1(ieiMICOIndication, m.MICOIndication.Nibble())
	}

	if m.T3512 != nil {
		raw, err := m.T3512.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3512Value, raw)
	}

	if m.Non3GppDeregistrationTimer != nil {
		raw, err := m.Non3GppDeregistrationTimer.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNon3GppDeregTimer, raw)
	}

	if m.T3502 != nil {
		raw, err := m.T3502.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3502Value, raw)
	}

	if m.SORTransparentContainer != nil {
		o.TLVE(ieiSORContainer, m.SORTransparentContainer)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	if m.NegotiatedDRX != nil {
		raw, err := m.NegotiatedDRX.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNegotiatedDRX, raw)
	}

	if m.EPSBearerContextStatus != nil {
		raw, err := m.EPSBearerContextStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiEPSBearerContextStatus, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *RegistrationAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// RegistrationComplete is the REGISTRATION COMPLETE message (TS 24.501 §8.2.8):
// an optional SOR transparent container the UE echoes back to the network.
type RegistrationComplete struct {
	SORTransparentContainer []byte // optional (IEI 0x73)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var registrationCompleteIEs = []nas.OptionalIE{
	{IEI: ieiSORContainer, Format: nas.IETLVE, Name: "SOR container"},
}

// AppendBinary encodes the plain REGISTRATION COMPLETE message.
// The encoding is appended to b.
func (m *RegistrationComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgRegistrationComplete)

	if m.SORTransparentContainer != nil {
		o.TLVE(ieiSORContainer, m.SORTransparentContainer)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *RegistrationComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseRegistrationComplete decodes the message.
func ParseRegistrationComplete(b []byte) (*RegistrationComplete, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgRegistrationComplete); err != nil {
		return nil, err
	}

	out := &RegistrationComplete{}

	_unrec, err := walkOptionalIEs(r, registrationCompleteIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiSORContainer {
			return false, nil
		}

		out.SORTransparentContainer = value

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// NetworkFeatureSupport is the 5GS network feature support IE (TS 24.501
// §9.11.3.5): a 1-to-3 octet content whose fields report the network's emergency,
// IMS-voice and interworking capabilities. Octets beyond those present read as
// zero.
type NetworkFeatureSupport struct {
	IMSVoPS3GPP  bool  // IMS voice over PS session over 3GPP access (octet 3, bit 1)
	IMSVoPSN3GPP bool  // IMS voice over PS session over non-3GPP access (octet 3, bit 2)
	EMC          uint8 // emergency service support (octet 3, bits 3-4)
	EMF          uint8 // emergency service fallback (octet 3, bits 5-6)
	IWKN26       bool  // interworking without N26 (octet 3, bit 7)
	MPSI         bool  // MPS indicator (octet 3, bit 8)
	EMCN3        bool  // emergency service support over non-3GPP (octet 4, bit 1)
	MCSI         bool  // MCS indicator (octet 4, bit 2)

	// HasOctet4 records whether the sender included octet 4, so the element
	// re-encodes at the length it arrived with; Octet4Spare and Rest carry the
	// bits and octets this codec does not interpret.
	HasOctet4   bool
	Octet4Spare uint8
	Rest        []byte
}

// maxNetworkFeatureSupportLen is the element's longest value: TS 24.501
// §9.11.3.5 caps the whole element at 6 octets, two of which are its IEI and
// length.
const maxNetworkFeatureSupportLen = 4

// ParseNetworkFeatureSupport decodes a 5GS network feature support IE value.
func ParseNetworkFeatureSupport(b []byte) (NetworkFeatureSupport, error) {
	if len(b) < 1 {
		return NetworkFeatureSupport{}, fmt.Errorf("nas/fgs: empty network feature support")
	}

	if len(b) > maxNetworkFeatureSupportLen {
		return NetworkFeatureSupport{}, fmt.Errorf(
			"nas/fgs: network feature support is %d octets, want at most %d", len(b), maxNetworkFeatureSupportLen)
	}

	out := NetworkFeatureSupport{
		IMSVoPS3GPP:  b[0]&0x01 != 0,
		IMSVoPSN3GPP: b[0]>>1&0x01 != 0,
		EMC:          b[0] >> 2 & 0x03,
		EMF:          b[0] >> 4 & 0x03,
		IWKN26:       b[0]>>6&0x01 != 0,
		MPSI:         b[0]>>7&0x01 != 0,
	}

	if len(b) >= 2 {
		out.HasOctet4 = true
		out.EMCN3 = b[1]&0x01 != 0
		out.MCSI = b[1]>>1&0x01 != 0
		out.Octet4Spare = b[1] >> 2 & 0x3F
	}

	if len(b) > 2 {
		out.Rest = bytes.Clone(b[2:])
	}

	return out, nil
}

// AppendBinary encodes the network feature support IE value as two octets.
// The encoding is appended to b.
func (n NetworkFeatureSupport) AppendBinary(b []byte) ([]byte, error) {
	if !n.HasOctet4 && (n.EMCN3 || n.MCSI || n.Octet4Spare != 0 || len(n.Rest) > 0) {
		return b, fmt.Errorf("nas/fgs: network feature support: octet 4 content requires octet 4")
	}

	if n2 := 2 + len(n.Rest); n.HasOctet4 && n2 > maxNetworkFeatureSupportLen {
		return b, fmt.Errorf(
			"nas/fgs: network feature support is %d octets, want at most %d", n2, maxNetworkFeatureSupportLen)
	}

	octet3 := boolBit(n.IMSVoPS3GPP, 0) | boolBit(n.IMSVoPSN3GPP, 1) |
		(n.EMC&0x03)<<2 | (n.EMF&0x03)<<4 | boolBit(n.IWKN26, 6) | boolBit(n.MPSI, 7)
	b = append(b, octet3)

	if !n.HasOctet4 {
		return b, nil
	}

	b = append(b, boolBit(n.EMCN3, 0)|boolBit(n.MCSI, 1)|n.Octet4Spare&0x3F<<2)

	return append(b, n.Rest...), nil
}

// MarshalBinary encodes the NetworkFeatureSupport information element value.
func (n NetworkFeatureSupport) MarshalBinary() ([]byte, error) { return n.AppendBinary(nil) }

// boolBit returns 1<<pos when set, else 0.
func boolBit(set bool, pos uint8) uint8 {
	if set {
		return 1 << pos
	}

	return 0
}

// UEStatus is the UE status information element (TS 24.501 §9.11.3.56): the
// UE's registration state in each system, which is what tells the network a
// registration is an inter-system move.
type UEStatus struct {
	// S1ModeReg reports the UE is in EMM-REGISTERED state (octet 3, bit 1).
	S1ModeReg bool
	// N1ModeReg reports the UE is in 5GMM-REGISTERED state (octet 3, bit 2).
	N1ModeReg bool
}

// ParseUEStatus decodes a UE status IE value.
func ParseUEStatus(b []byte) (UEStatus, error) {
	if len(b) != 1 {
		return UEStatus{}, fmt.Errorf("nas/fgs: UE status is %d octets, want 1", len(b))
	}

	return UEStatus{S1ModeReg: b[0]&0x01 != 0, N1ModeReg: b[0]&0x02 != 0}, nil
}

// MarshalBinary encodes the UE status IE value.
func (u UEStatus) MarshalBinary() []byte {
	return []byte{boolBit(u.S1ModeReg, 0) | boolBit(u.N1ModeReg, 1)}
}
