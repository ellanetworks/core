// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// EMM tracking area updating message types (TS 24.301).
const (
	MsgTrackingAreaUpdateRequest  MessageType = 0x48
	MsgTrackingAreaUpdateAccept   MessageType = 0x49
	MsgTrackingAreaUpdateComplete MessageType = 0x4a
	MsgTrackingAreaUpdateReject   MessageType = 0x4b
)

// EPS update result values (TS 24.301).
const (
	EPSUpdateResultTA       uint8 = 0
	EPSUpdateResultCombined uint8 = 1
)

// EPS update type values (TS 24.301).
const (
	EPSUpdateTypeTA       uint8 = 0
	EPSUpdateTypePeriodic uint8 = 3
)

// TrackingAreaUpdateRequest is the UE's request to update its registration
// (TS 24.301). The mandatory EPS update type half-octet is decoded (the
// active flag drives whether the network re-establishes the radio bearer); the
// optional EPS bearer context status, when present, reports which EPS bearers the
// UE still considers active so the network can reconcile.
type TrackingAreaUpdateRequest struct {
	EPSUpdateType uint8 // EPS update type value (bits 1-3, TS 24.301)
	ActiveFlag    bool  // bit 4: bearer establishment requested
	NASKeySetID   uint8
	// EPSBearerContextStatus is the EBI activity bitmap (bit n = EBI n active),
	// nil when the UE did not include the IE (IEI 0x57, TS 24.301).
	EPSBearerContextStatus *uint16
}

// epsBearerContextStatusIEI is the IEI of the EPS bearer context status IE in the
// TRACKING AREA UPDATE REQUEST and ACCEPT (TS 24.301).
const epsBearerContextStatusIEI = 0x57

// tauRequestOptionalIEs lists the full-octet optional IEs of the TRACKING AREA
// UPDATE REQUEST (TS 24.301), transcribed so the walker can delimit every
// IE and reach the EPS bearer context status regardless of what precedes it.
// Type-1/2 IEs (IEI ≥ 0x80) are delimited generically and are not listed.
var tauRequestOptionalIEs = []common.OptionalIE{
	{IEI: 0x19, Format: common.IETV3, Len: 3}, // Old P-TMSI signature
	{IEI: 0x50, Format: common.IETLV},         // Additional GUTI
	{IEI: 0x55, Format: common.IETV3, Len: 4}, // NonceUE
	{IEI: 0x58, Format: common.IETLV},         // UE network capability
	{IEI: 0x52, Format: common.IETV3, Len: 5}, // Last visited registered TAI
	{IEI: 0x5C, Format: common.IETV3, Len: 2}, // DRX parameter
	{IEI: epsBearerContextStatusIEI, Format: common.IETLV},
	{IEI: 0x31, Format: common.IETLV},         // MS network capability
	{IEI: 0x13, Format: common.IETV3, Len: 5}, // Old location area identification
	{IEI: 0x11, Format: common.IETLV},         // Mobile station classmark 2
	{IEI: 0x20, Format: common.IETLV},         // Mobile station classmark 3
	{IEI: 0x40, Format: common.IETLV},         // Supported codecs
	{IEI: 0x5D, Format: common.IETLV},         // Voice domain preference
	{IEI: 0x10, Format: common.IETLV},         // TMSI based NRI container
	{IEI: 0x6A, Format: common.IETLV},         // T3324 value
	{IEI: 0x5E, Format: common.IETLV},         // T3412 extended value
	{IEI: 0x6E, Format: common.IETLV},         // Extended DRX parameters
	{IEI: 0x6F, Format: common.IETLV},         // UE additional security capability
	{IEI: 0x6D, Format: common.IETLV},         // UE status
	{IEI: 0x17, Format: common.IETV3, Len: 1}, // Additional information requested
	{IEI: 0x32, Format: common.IETLV},         // N1 UE network capability
	{IEI: 0x34, Format: common.IETLV},         // UE radio capability ID
	{IEI: 0x35, Format: common.IETLV},         // Requested WUS assistance information
}

// encodeEPSBearerContextStatus encodes the EBI activity bitmap into the two-octet
// value (octet 1 = EBI 0-7, octet 2 = EBI 8-15; bit n = EBI n, TS 24.301).
func encodeEPSBearerContextStatus(status uint16) []byte {
	return []byte{byte(status), byte(status >> 8)}
}

// parseEPSBearerContextStatus decodes the two-octet EBI activity bitmap.
func parseEPSBearerContextStatus(value []byte) (uint16, error) {
	if len(value) < 2 {
		return 0, fmt.Errorf("nas/eps: EPS bearer context status too short: %d", len(value))
	}

	return uint16(value[0]) | uint16(value[1])<<8, nil
}

// Marshal encodes the plain TRACKING AREA UPDATE REQUEST. Only the mandatory EPS
// update type octet (NAS key set identifier | active flag | update type) is
// written; the UE is resolved from the S-TMSI in the S1AP Initial UE Message, so
// the old GUTI is omitted (the receiver here ignores it, TS 24.301).
func (m *TrackingAreaUpdateRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgTrackingAreaUpdateRequest)

	octet := (m.NASKeySetID&0x07)<<4 | (m.EPSUpdateType & 0x07)
	if m.ActiveFlag {
		octet |= 0x08
	}

	w.U8(octet)

	if m.EPSBearerContextStatus != nil {
		w.U8(epsBearerContextStatusIEI)

		if err := w.LV(encodeEPSBearerContextStatus(*m.EPSBearerContextStatus)); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseTrackingAreaUpdateRequest decodes a plain TRACKING AREA UPDATE REQUEST.
// The old GUTI and optional IEs are not decoded; the UE is resolved from the
// S-TMSI carried in the S1AP Initial UE Message.
func ParseTrackingAreaUpdateRequest(b []byte) (*TrackingAreaUpdateRequest, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &TrackingAreaUpdateRequest{
		EPSUpdateType: octet & 0x07,
		ActiveFlag:    octet&0x08 != 0,
		NASKeySetID:   (octet >> 4) & 0x07,
	}

	if _, err := common.WalkOptionalIEs(r, tauRequestOptionalIEs, func(iei uint8, value []byte) error {
		if iei != epsBearerContextStatusIEI {
			return nil
		}

		status, err := parseEPSBearerContextStatus(value)
		if err != nil {
			return err
		}

		m.EPSBearerContextStatus = &status

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// taiListIEI is the IEI of the optional TAI list information element in
// TRACKING AREA UPDATE ACCEPT (TS 24.301).
const taiListIEI = 0x54

// TrackingAreaUpdateAccept accepts a tracking area updating procedure
// (TS 24.301). The mandatory EPS update result is encoded; the optional
// GUTI, TAI list, EMM cause, and EPS network feature support follow when present.
type TrackingAreaUpdateAccept struct {
	EPSUpdateResult uint8
	GUTI            *EPSMobileIdentity // reallocated GUTI (IEI 0x50), when present
	TAIList         []byte             // TAI list value (IEI 0x54), when present
	// EPSBearerContextStatus is the EBI activity bitmap the MME reports back when
	// the UE included one in the request (bit n = EBI n active, IEI 0x57,
	// TS 24.301); nil when the IE is absent.
	EPSBearerContextStatus *uint16
	EMMCause               *uint8 // EMM cause (IEI 0x53), when present
	// EPS network feature support (IEI 0x64), when present (TS 24.301).
	EPSNetworkFeatureSupport *EPSNetworkFeatureSupport
}

// tauAcceptIEs are the optional IEs Ella Core emits in a TRACKING AREA UPDATE
// ACCEPT (TS 24.301): the reallocated GUTI, the TAI list, the EPS bearer
// context status, the EMM cause, and the EPS network feature support. EMM cause
// is a type-3 IE with a one-octet value; the others are type-4 TLVs.
var tauAcceptIEs = []common.OptionalIE{
	{IEI: gutiIEI, Format: common.IETLV},
	{IEI: taiListIEI, Format: common.IETLV},
	{IEI: epsBearerContextStatusIEI, Format: common.IETLV},
	{IEI: emmCauseIEI, Format: common.IETV3, Len: 1},
	{IEI: epsNetworkFeatureSupportIEI, Format: common.IETLV},
}

// Marshal encodes the plain TRACKING AREA UPDATE ACCEPT message. A GUTI
// reallocates the UE's identity and is acknowledged with TRACKING AREA UPDATE
// COMPLETE (TS 24.301).
func (m *TrackingAreaUpdateAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgTrackingAreaUpdateAccept)
	w.U8(m.EPSUpdateResult & 0x07) // EPS update result | spare half octet

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

	if len(m.TAIList) > 0 {
		w.U8(taiListIEI)

		if err := w.LV(m.TAIList); err != nil {
			return nil, err
		}
	}

	if m.EPSBearerContextStatus != nil {
		w.U8(epsBearerContextStatusIEI)

		if err := w.LV(encodeEPSBearerContextStatus(*m.EPSBearerContextStatus)); err != nil {
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

// ParseTrackingAreaUpdateAccept decodes a plain TRACKING AREA UPDATE ACCEPT.
func ParseTrackingAreaUpdateAccept(b []byte) (*TrackingAreaUpdateAccept, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateAccept); err != nil {
		return nil, err
	}

	result, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &TrackingAreaUpdateAccept{EPSUpdateResult: result & 0x07}

	if _, err := common.WalkOptionalIEs(r, tauAcceptIEs, func(iei uint8, value []byte) error {
		switch iei {
		case gutiIEI:
			id, err := decodeEPSMobileIdentity(value)
			if err != nil {
				return err
			}

			m.GUTI = &id
		case taiListIEI:
			m.TAIList = value
		case epsBearerContextStatusIEI:
			status, err := parseEPSBearerContextStatus(value)
			if err != nil {
				return err
			}

			m.EPSBearerContextStatus = &status
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

// TrackingAreaUpdateComplete is the UE's acknowledgement of a TAU Accept that
// reallocated the GUTI (TS 24.301). It carries no information elements.
type TrackingAreaUpdateComplete struct{}

// Marshal encodes the plain TRACKING AREA UPDATE COMPLETE message.
func (m *TrackingAreaUpdateComplete) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgTrackingAreaUpdateComplete)

	return w.Bytes(), nil
}

// ParseTrackingAreaUpdateComplete decodes a plain TRACKING AREA UPDATE COMPLETE
// message.
func ParseTrackingAreaUpdateComplete(b []byte) (*TrackingAreaUpdateComplete, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateComplete); err != nil {
		return nil, err
	}

	return &TrackingAreaUpdateComplete{}, nil
}

// TrackingAreaUpdateReject is the network's rejection of a tracking area
// updating procedure (TS 24.301). With EMM cause #9 or #10 the UE
// accepts it without integrity protection and re-attaches.
type TrackingAreaUpdateReject struct {
	Cause uint8
}

// Marshal encodes the plain TRACKING AREA UPDATE REJECT message.
func (m *TrackingAreaUpdateReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgTrackingAreaUpdateReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// ParseTrackingAreaUpdateReject decodes a plain TRACKING AREA UPDATE REJECT
// message.
func ParseTrackingAreaUpdateReject(b []byte) (*TrackingAreaUpdateReject, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgTrackingAreaUpdateReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &TrackingAreaUpdateReject{Cause: cause}, nil
}
