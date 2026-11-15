// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// UE-NGAP-IDs ::= CHOICE { uE-NGAP-ID-pair, aMF-UE-NGAP-ID, choice-Extensions }.
type UENGAPIDs struct {
	UENGAPIDPair    *UENGAPIDPair    `per:",choice:0"`
	AMFUENGAPID     *AMFUENGAPID     `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type UENGAPIDPair struct {
	AMFUENGAPID  AMFUENGAPID
	RANUENGAPID  RANUENGAPID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PDUSessionResourceItemCxtRelReq ::= SEQUENCE { pDUSessionID, iE-Extensions
// OPTIONAL } (extensible).
type PDUSessionResourceItemCxtRelReq struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	PDUSessionID PDUSessionID
	iEExtensions extensionMeta `per:",exttable"`
}

// PDUSessionResourceListCxtRelReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions)) OF
// PDUSessionResourceItemCxtRelReq. The sessions the NG-RAN node wants released
// with the connection (§9.2.2.4).
type PDUSessionResourceListCxtRelReq []PDUSessionResourceItemCxtRelReq

// PDUSessionResourceItemCxtRelCpl ::= SEQUENCE { pDUSessionID, iE-Extensions
// OPTIONAL } (extensible).
type PDUSessionResourceItemCxtRelCpl struct {
	extensions                                per.ExtensionAdditions `per:",extraw"`
	PDUSessionID                              PDUSessionID
	PDUSessionResourceReleaseResponseTransfer *TransferContainer `per:",extie:IDPDUSessionResourceReleaseResponseTransfer,crit:ignore"`
	iEExtensions                              extensionMeta      `per:",exttable"`
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
		ID: IDUENGAPIDs, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UENGAPIDs)
		},
		Encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) { return &m.UENGAPIDs, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseCommand, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) {
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

func DecodeUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	return decodeMessageBody[UEContextReleaseCommand](ProcUEContextRelease, uEContextReleaseCommandIEs, value)
}

// TS 38.413 §9.2.2.6.
type UEContextReleaseComplete struct {
	AMFUENGAPID                                *AMFUENGAPID
	RANUENGAPID                                *RANUENGAPID
	UserLocationInformation                    *UserLocationInformation
	PDUSessionResourceList                     PDUSessionResourceListCxtRelCpl
	CriticalityDiagnostics                     *CriticalityDiagnostics
	InfoOnRecommendedCellsAndRANNodesForPaging *InfoOnRecommendedCellsAndRANNodesForPaging
	PagingAssisDataforCEcapabUE                *PagingAssisDataforCEcapabUE

	messageMeta
}

var uEContextReleaseCompleteIEs = []ieSpec[UEContextReleaseComplete]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	apimpl.OptionalIE(IDInfoOnRecommendedCellsAndRANNodesForPaging, CriticalityIgnore, func(m *UEContextReleaseComplete) **InfoOnRecommendedCellsAndRANNodesForPaging {
		return &m.InfoOnRecommendedCellsAndRANNodesForPaging
	}),
	{
		ID: IDPDUSessionResourceListCxtRelCpl, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceList)
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.PDUSessionResourceList == nil {
				return nil, false
			}

			return m.PDUSessionResourceList, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	apimpl.OptionalIE(IDPagingAssisDataforCEcapabUE, CriticalityIgnore, func(m *UEContextReleaseComplete) **PagingAssisDataforCEcapabUE { return &m.PagingAssisDataforCEcapabUE }),
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

func DecodeUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	return decodeMessageBody[UEContextReleaseComplete](ProcUEContextRelease, uEContextReleaseCompleteIEs, value)
}

// TS 38.413 §9.2.2.4.
type UEContextReleaseRequest struct {
	AMFUENGAPID                AMFUENGAPID
	RANUENGAPID                RANUENGAPID
	PDUSessionResourceList     PDUSessionResourceListCxtRelReq
	Cause                      *Cause
	GWContextReleaseIndication *GWContextReleaseIndication

	messageMeta
}

var uEContextReleaseRequestIEs = []ieSpec[UEContextReleaseRequest]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDPDUSessionResourceListCxtRelReq, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.PDUSessionResourceList)
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceList == nil {
				return nil, false
			}

			return m.PDUSessionResourceList, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDGWContextReleaseIndication, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			var v GWContextReleaseIndication

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GWContextReleaseIndication = &v

			return nil
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.GWContextReleaseIndication == nil {
				return nil, false
			}

			return m.GWContextReleaseIndication, true
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

func DecodeUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	return decodeMessageBody[UEContextReleaseRequest](ProcUEContextReleaseRequest, uEContextReleaseRequestIEs, value)
}
