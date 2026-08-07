// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/nas"
)

// EPS update result values (TS 24.301).
const (
	EPSUpdateResultTA                   EPSUpdateResult = 0
	EPSUpdateResultCombined             EPSUpdateResult = 1
	EPSUpdateResultTAISRActivated       EPSUpdateResult = 4
	EPSUpdateResultCombinedISRActivated EPSUpdateResult = 5
)

// EPS update type values (TS 24.301).
const (
	EPSUpdateTypeTA               EPSUpdateType = 0
	EPSUpdateTypeCombinedTALA     EPSUpdateType = 1
	EPSUpdateTypeCombinedTALAIMSI EPSUpdateType = 2
	EPSUpdateTypePeriodic         EPSUpdateType = 3
)

// TrackingAreaUpdateRequest is the UE's request to update its registration
// (TS 24.301). The mandatory EPS update type half-octet is decoded (the
// active flag drives whether the network re-establishes the radio bearer); the
// optional EPS bearer context status, when present, reports which EPS bearers the
// UE still considers active so the network can reconcile.
type TrackingAreaUpdateRequest struct {
	EPSUpdateType       EPSUpdateType // EPS update type value (bits 1-3, TS 24.301)
	ActiveFlag          bool          // bit 4: bearer establishment requested
	NASKeySetIdentifier nas.KeySetIdentifier
	// OldGUTI is the mandatory Old GUTI (EPS mobile identity, LV) that follows the
	// NAS key set identifier half-octet — TS 24.301 table 8.2.29.1.
	OldGUTI EPSMobileIdentity
	// EPSBearerContextStatus reports which EPS bearer contexts are active.
	EPSBearerContextStatus *nas.EPSBearerContextStatus
	UENetworkCapability    *UENetworkCapability // §9.9.3.34
	MSNetworkCapability    *MSNetworkCapability // §9.9.3.20

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// tauRequestOptionalIEs lists the full-octet optional IEs of the TRACKING AREA
// UPDATE REQUEST (TS 24.301), transcribed so the walker can delimit every
// IE and reach the EPS bearer context status regardless of what precedes it.
// Type-1/2 IEs (IEI ≥ 0x80) are delimited generically and are not listed.
var tauRequestOptionalIEs = []nas.OptionalIE{
	{IEI: ieiOldPTMSISignature, Format: nas.IETV3, Len: 3, Name: "Old P-TMSI signature"},
	{IEI: ieiAdditionalGUTI, Format: nas.IETLV, Name: "Additional GUTI"},
	{IEI: ieiNonceUE, Format: nas.IETV3, Len: 4, Name: "Nonce UE"},
	{IEI: ieiUENetworkCapability, Format: nas.IETLV, Critical: true, Name: "UE network capability"},
	{IEI: ieiLastVisitedRegisteredTAI, Format: nas.IETV3, Len: 5, Name: "Last visited registered TAI"},
	{IEI: ieiDRXParameter, Format: nas.IETV3, Len: 2, Name: "DRX parameter"},
	{IEI: ieiEPSBearerContextStatus, Format: nas.IETLV, Name: "EPS bearer context status"},
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
}

// AppendBinary encodes the plain TRACKING AREA UPDATE REQUEST. Only the mandatory EPS
// update type octet (NAS key set identifier | active flag | update type) is
// written; the UE is resolved from the S-TMSI in the S1AP Initial UE Message, so
// the old GUTI is omitted (the receiver here ignores it, TS 24.301).
// The encoding is appended to b.
func (m *TrackingAreaUpdateRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgTrackingAreaUpdateRequest)

	octet := m.NASKeySetIdentifier.HalfOctet()<<4 | (uint8(m.EPSUpdateType) & 0x07)
	if m.ActiveFlag {
		octet |= 0x08
	}

	w.U8(octet)

	// Mandatory Old GUTI (EPS mobile identity, LV) — TS 24.301 table 8.2.29.1.
	oldGUTI, err := m.OldGUTI.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(oldGUTI)

	if m.UENetworkCapability != nil {
		raw, err := m.UENetworkCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiUENetworkCapability, raw)
	}

	if m.EPSBearerContextStatus != nil {
		raw, err := m.EPSBearerContextStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiEPSBearerContextStatus, raw)
	}

	if m.MSNetworkCapability != nil {
		raw, err := m.MSNetworkCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiMSNetworkCapability, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *TrackingAreaUpdateRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseTrackingAreaUpdateRequest decodes a plain TRACKING AREA UPDATE REQUEST.
// The old GUTI and optional IEs are not decoded; the UE is resolved from the
// S-TMSI carried in the S1AP Initial UE Message.
func ParseTrackingAreaUpdateRequest(b []byte) (*TrackingAreaUpdateRequest, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &TrackingAreaUpdateRequest{
		EPSUpdateType:       EPSUpdateType(octet & 0x07),
		ActiveFlag:          octet&0x08 != 0,
		NASKeySetIdentifier: nas.ParseKeySetIdentifier(octet >> 4),
	}

	// Mandatory Old GUTI (EPS mobile identity, LV) precedes every optional IE
	// (TS 24.301 table 8.2.29.1); consume it so the optional-IE walk starts aligned.
	oldGUTIRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.OldGUTI, err = ParseEPSMobileIdentity(oldGUTIRaw); err != nil {
		return nil, err
	}

	_unrec, err := walkOptionalIEs(r, tauRequestOptionalIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiEPSBearerContextStatus:
			status, err := nas.ParseEPSBearerContextStatus(value)
			if err != nil {
				return false, err
			}

			m.EPSBearerContextStatus = &status

			return true, nil
		case ieiUENetworkCapability:
			// Both are Critical, so a malformed value fails the message rather than
			// taking the §7.7.1 not-present default.
			parsed, err := ParseUENetworkCapability(value)
			if err != nil {
				return false, err
			}

			m.UENetworkCapability = &parsed

			return true, nil
		case ieiMSNetworkCapability:
			parsed, err := ParseMSNetworkCapability(value)
			if err != nil {
				return false, err
			}

			m.MSNetworkCapability = &parsed

			return true, nil
		}

		return false, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// ieiTAIList is the IEI of the optional TAI list information element in
// TRACKING AREA UPDATE ACCEPT (TS 24.301).
const ieiTAIList = 0x54

// TrackingAreaUpdateAccept accepts a tracking area updating procedure
// (TS 24.301). The mandatory EPS update result is encoded; the optional
// GUTI, TAI list, EMM cause, and EPS network feature support follow when present.
type TrackingAreaUpdateAccept struct {
	EPSUpdateResult EPSUpdateResult
	GUTI            *EPSMobileIdentity // reallocated GUTI (IEI 0x50), when present
	TAIList         *TAIList           // TAI list value (IEI 0x54), when present
	// EPSBearerContextStatus reports which EPS bearer contexts the network holds
	// active.
	EPSBearerContextStatus *nas.EPSBearerContextStatus
	Cause                  *EMMCause // EMM cause (IEI 0x53), when present
	// EPS network feature support (IEI 0x64), when present (TS 24.301).
	NetworkFeatureSupport *NetworkFeatureSupport

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// tauAcceptIEs are the optional IEs Ella Core emits in a TRACKING AREA UPDATE
// ACCEPT (TS 24.301): the reallocated GUTI, the TAI list, the EPS bearer
// context status, the EMM cause, and the EPS network feature support. EMM cause
// is a type-3 IE with a one-octet value; the others are type-4 TLVs.
var tauAcceptIEs = []nas.OptionalIE{
	{IEI: ieiT3412Value, Format: nas.IETV3, Len: 1, Name: "T3412 value"},
	{IEI: ieiGUTI, Format: nas.IETLV, Name: "GUTI"},
	{IEI: ieiTAIList, Format: nas.IETLV, Name: "TAI list"},
	{IEI: ieiEPSBearerContextStatus, Format: nas.IETLV, Name: "EPS bearer context status"},
	{IEI: ieiLocationAreaID, Format: nas.IETV3, Len: 5, Name: "Location area identification"},
	{IEI: ieiEMMCause, Format: nas.IETV3, Len: 1, Name: "EMM cause"},
	{IEI: ieiT3402ValueAccept, Format: nas.IETV3, Len: 1, Name: "T3402 value"},
	{IEI: ieiT3423Value, Format: nas.IETV3, Len: 1, Name: "T3423 value"},
	{IEI: ieiNetworkFeatureSupport, Format: nas.IETLV, Name: "Network feature support"},
}

// AppendBinary encodes the plain TRACKING AREA UPDATE ACCEPT message. A GUTI
// reallocates the UE's identity and is acknowledged with TRACKING AREA UPDATE
// COMPLETE (TS 24.301).
// The encoding is appended to b.
func (m *TrackingAreaUpdateAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgTrackingAreaUpdateAccept)
	w.U8(uint8(m.EPSUpdateResult & 0x07)) // EPS update result | spare half octet

	if m.GUTI != nil {
		raw, err := m.GUTI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiGUTI, raw)
	}

	if m.TAIList != nil {
		raw, err := m.TAIList.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiTAIList, raw)
	}

	if m.EPSBearerContextStatus != nil {
		raw, err := m.EPSBearerContextStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiEPSBearerContextStatus, raw)
	}

	if m.Cause != nil {
		o.TV3(ieiEMMCause, []byte{uint8(*m.Cause)})
	}

	if m.NetworkFeatureSupport != nil {
		raw, err := m.NetworkFeatureSupport.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNetworkFeatureSupport, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *TrackingAreaUpdateAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseTrackingAreaUpdateAccept decodes a plain TRACKING AREA UPDATE ACCEPT.
func ParseTrackingAreaUpdateAccept(b []byte) (*TrackingAreaUpdateAccept, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateAccept); err != nil {
		return nil, err
	}

	result, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &TrackingAreaUpdateAccept{EPSUpdateResult: EPSUpdateResult(result & 0x07)}

	_unrec, err := walkOptionalIEs(r, tauAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiGUTI:
			parsed, err := ParseEPSMobileIdentity(value)
			if err != nil {
				return false, err
			}

			m.GUTI = &parsed
		case ieiTAIList:
			parsed, err := ParseTAIList(value)
			if err != nil {
				return false, err
			}

			m.TAIList = &parsed
		case ieiEPSBearerContextStatus:
			status, err := nas.ParseEPSBearerContextStatus(value)
			if err != nil {
				return false, err
			}

			m.EPSBearerContextStatus = &status
		case ieiEMMCause:
			if len(value) == 0 {
				return false, nil
			}

			cause := EMMCause(value[0])
			m.Cause = &cause
		case ieiNetworkFeatureSupport:
			parsed, err := ParseNetworkFeatureSupport(value)
			if err != nil {
				return false, err
			}

			m.NetworkFeatureSupport = &parsed
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

// TrackingAreaUpdateComplete is the UE's acknowledgement of a TAU Accept that
// reallocated the GUTI (TS 24.301). It carries no information elements.
type TrackingAreaUpdateComplete struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain TRACKING AREA UPDATE COMPLETE message.
// The encoding is appended to b.
func (m *TrackingAreaUpdateComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgTrackingAreaUpdateComplete)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *TrackingAreaUpdateComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseTrackingAreaUpdateComplete decodes a plain TRACKING AREA UPDATE COMPLETE
// message.
func ParseTrackingAreaUpdateComplete(b []byte) (*TrackingAreaUpdateComplete, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateComplete); err != nil {
		return nil, err
	}

	out := &TrackingAreaUpdateComplete{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// TrackingAreaUpdateReject is the network's rejection of a tracking area
// updating procedure (TS 24.301). With EMM cause #9 or #10 the UE
// accepts it without integrity protection and re-attaches.
type TrackingAreaUpdateReject struct {
	Cause EMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain TRACKING AREA UPDATE REJECT message.
// The encoding is appended to b.
func (m *TrackingAreaUpdateReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgTrackingAreaUpdateReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *TrackingAreaUpdateReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseTrackingAreaUpdateReject decodes a plain TRACKING AREA UPDATE REJECT
// message.
func ParseTrackingAreaUpdateReject(b []byte) (*TrackingAreaUpdateReject, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &TrackingAreaUpdateReject{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
