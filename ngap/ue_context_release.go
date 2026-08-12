// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// UE-NGAP-IDs CHOICE alternatives. The CHOICE is not extensible, so
// choice-Extensions is a plain alternative index.
const (
	ueNGAPIDsPair = iota
	ueNGAPIDsAMFUENGAPID
	ueNGAPIDsChoiceExtensions

	ueNGAPIDsAlternatives = 3
)

// UE-NGAP-IDs ::= CHOICE { uE-NGAP-ID-pair, aMF-UE-NGAP-ID, choice-Extensions }.
// Pair selects uE-NGAP-ID-pair, which the AMF sends once the NG-RAN node has
// assigned a RAN UE NGAP ID; otherwise only AMFUENGAPID is carried.
type UENGAPIDs struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	Pair        bool
}

func (u UENGAPIDs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if !u.Pair {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ueNGAPIDsAlternatives-1, ueNGAPIDsAMFUENGAPID); err != nil {
			return err
		}

		return u.AMFUENGAPID.MarshalPER(w, enc)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ueNGAPIDsAlternatives-1, ueNGAPIDsPair); err != nil {
		return err
	}

	// UE-NGAP-ID-pair ::= SEQUENCE { aMF-UE-NGAP-ID, rAN-UE-NGAP-ID,
	// iE-Extensions OPTIONAL } (extensible).
	w.WriteBit(false)
	w.WriteBit(false)

	if err := u.AMFUENGAPID.MarshalPER(w, enc); err != nil {
		return err
	}

	return u.RANUENGAPID.MarshalPER(w, enc)
}

func (u *UENGAPIDs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, ueNGAPIDsAlternatives-1)
	if err != nil {
		return err
	}

	switch idx {
	case ueNGAPIDsPair:
		extBit, err := r.ReadBit()
		if err != nil {
			return err
		}

		extContainer, err := r.ReadBit()
		if err != nil {
			return err
		}

		var amf AMFUENGAPID
		if err := amf.UnmarshalPER(r, enc); err != nil {
			return err
		}

		var ran RANUENGAPID
		if err := ran.UnmarshalPER(r, enc); err != nil {
			return err
		}

		if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
			return err
		}

		*u = UENGAPIDs{AMFUENGAPID: amf, RANUENGAPID: ran, Pair: true}

		return nil
	case ueNGAPIDsChoiceExtensions:
		return decodeChoiceExtension(r, enc, "UE-NGAP-IDs")
	default:
		var amf AMFUENGAPID
		if err := amf.UnmarshalPER(r, enc); err != nil {
			return err
		}

		*u = UENGAPIDs{AMFUENGAPID: amf}

		return nil
	}
}

// PDUSessionResourceItemCxtRelReq ::= SEQUENCE { pDUSessionID, iE-Extensions
// OPTIONAL } (extensible).
type PDUSessionResourceItemCxtRelReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceListCxtRelReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceItemCxtRelReq. The sessions the NG-RAN node wants released
// with the connection (§9.2.2.4).
type PDUSessionResourceListCxtRelReq []PDUSessionResourceItemCxtRelReq

// PDUSessionResourceItemCxtRelCpl ::= SEQUENCE { pDUSessionID, iE-Extensions
// OPTIONAL } (extensible). The release response transfer rides in an
// ignore-criticality extension, so it is skipped with the container.
type PDUSessionResourceItemCxtRelCpl struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceListCxtRelCpl ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceItemCxtRelCpl.
//
// Ella Core releases a UE's sessions from its own state rather than from this
// list, but the IE is reject criticality: not comprehending it would reject a
// UE CONTEXT RELEASE COMPLETE from any NG-RAN node with an active session
// (§10.3.4.2).
type PDUSessionResourceListCxtRelCpl []PDUSessionResourceItemCxtRelCpl

// TS 38.413 §9.2.2.5.
type UEContextReleaseCommand struct {
	UENGAPIDs UENGAPIDs
	Cause     *Cause

	messageMeta
}

var uEContextReleaseCommandIEs = []ieSpec[UEContextReleaseCommand]{
	{
		id: IDUENGAPIDs, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UENGAPIDs)
		},
		encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) { return &m.UENGAPIDs, true },
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseCommand, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *UEContextReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextRelease, uEContextReleaseCommandIEs, m)
}

func (m *UEContextReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	return parseMessageBody[UEContextReleaseCommand](ProcUEContextRelease, TriggeringInitiatingMessage, uEContextReleaseCommandIEs, value)
}

// TS 38.413 §9.2.2.6.
type UEContextReleaseComplete struct {
	AMFUENGAPID             *AMFUENGAPID
	RANUENGAPID             *RANUENGAPID
	UserLocationInformation *UserLocationInformation
	PDUSessionResourceList  PDUSessionResourceListCxtRelCpl
	CriticalityDiagnostics  *CriticalityDiagnostics

	messageMeta
}

var uEContextReleaseCompleteIEs = []ieSpec[UEContextReleaseComplete]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: IDPDUSessionResourceListCxtRelCpl, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceList)
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.PDUSessionResourceList == nil {
				return nil, false
			}

			return m.PDUSessionResourceList, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *UEContextReleaseComplete) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextRelease, uEContextReleaseCompleteIEs, m)
}

func (m *UEContextReleaseComplete) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	return parseMessageBody[UEContextReleaseComplete](ProcUEContextRelease, TriggeringSuccessfulOutcome, uEContextReleaseCompleteIEs, value)
}

// TS 38.413 §9.2.2.4.
type UEContextReleaseRequest struct {
	AMFUENGAPID            AMFUENGAPID
	RANUENGAPID            RANUENGAPID
	PDUSessionResourceList PDUSessionResourceListCxtRelReq
	Cause                  *Cause

	messageMeta
}

var uEContextReleaseRequestIEs = []ieSpec[UEContextReleaseRequest]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDPDUSessionResourceListCxtRelReq, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceList)
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceList == nil {
				return nil, false
			}

			return m.PDUSessionResourceList, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *UEContextReleaseRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextReleaseRequest, uEContextReleaseRequestIEs, m)
}

func (m *UEContextReleaseRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextReleaseRequest,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	return parseMessageBody[UEContextReleaseRequest](ProcUEContextReleaseRequest, TriggeringInitiatingMessage, uEContextReleaseRequestIEs, value)
}
