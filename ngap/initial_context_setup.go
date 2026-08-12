// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceSetupItemCxtReq ::= SEQUENCE { pDUSessionID, nAS-PDU
// OPTIONAL, s-NSSAI, pDUSessionResourceSetupRequestTransfer, iE-Extensions
// OPTIONAL } (extensible).
//
// The transfer is OCTET STRING (CONTAINING PDUSessionResourceSetupRequestTransfer)
// (§9.3.4.1). The AMF neither builds nor reads it — the SMF does — so it is
// carried opaquely.
type PDUSessionResourceSetupItemCxtReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	NASPDU       *NASPDU `per:",optional"`
	SNSSAI       SNSSAI
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListCxtReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemCxtReq.
type PDUSessionResourceSetupListCxtReq []PDUSessionResourceSetupItemCxtReq

// PDUSessionResourceSetupItemCxtRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupResponseTransfer, iE-Extensions OPTIONAL }
// (extensible). The transfer contains a PDUSessionResourceSetupResponseTransfer
// (§9.3.4.2).
type PDUSessionResourceSetupItemCxtRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListCxtRes ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemCxtRes.
type PDUSessionResourceSetupListCxtRes []PDUSessionResourceSetupItemCxtRes

// PDUSessionResourceFailedToSetupItemCxtRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible). The transfer contains a
// PDUSessionResourceSetupUnsuccessfulTransfer (§9.3.4.16).
type PDUSessionResourceFailedToSetupItemCxtRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListCxtRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemCxtRes.
type PDUSessionResourceFailedToSetupListCxtRes []PDUSessionResourceFailedToSetupItemCxtRes

// PDUSessionResourceFailedToSetupItemCxtFail has the same shape as its CxtRes
// counterpart but its own ASN.1 type, so the two lists stay distinct.
type PDUSessionResourceFailedToSetupItemCxtFail struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListCxtFail ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemCxtFail.
type PDUSessionResourceFailedToSetupListCxtFail []PDUSessionResourceFailedToSetupItemCxtFail

// TS 38.413 §9.2.2.1.
type InitialContextSetupRequest struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	// C-ifPDUSessionResourceSetup: present exactly when PDUSessionResourceSetup
	// is (§9.2.2.1).
	UEAggregateMaximumBitRate  *UEAggregateMaximumBitRate
	GUAMI                      GUAMI
	PDUSessionResourceSetup    PDUSessionResourceSetupListCxtReq
	AllowedNSSAI               AllowedNSSAI
	UESecurityCapabilities     UESecurityCapabilities
	SecurityKey                SecurityKey
	UERadioCapability          UERadioCapability
	NASPDU                     *NASPDU
	UERadioCapabilityForPaging *UERadioCapabilityForPaging

	messageMeta
}

var initialContextSetupRequestIEs = []ieSpec[InitialContextSetupRequest]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDUEAggregateMaximumBitRate, presence: presenceConditional, crit: CriticalityReject,
		condition: func(m *InitialContextSetupRequest) bool { return m.PDUSessionResourceSetup != nil },
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			var v UEAggregateMaximumBitRate

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &v

			return nil
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: IDGUAMI, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.GUAMI)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.GUAMI, true },
	},
	{
		id: IDPDUSessionResourceSetupListCxtReq, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: IDAllowedNSSAI, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AllowedNSSAI)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.AllowedNSSAI == nil {
				return nil, false
			}

			return m.AllowedNSSAI, true
		},
	},
	{
		id: IDUESecurityCapabilities, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			return &m.UESecurityCapabilities, true
		},
	},
	{
		id: IDSecurityKey, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityKey)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) { return &m.SecurityKey, true },
	},
	{
		id: IDUERadioCapability, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UERadioCapability)
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	{
		id: IDNASPDU, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			var v NASPDU

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		id: IDUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupRequest, raw []byte, enc per.Encoding) error {
			var v UERadioCapabilityForPaging

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		encode: func(m *InitialContextSetupRequest) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
}

func (m *InitialContextSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupRequestIEs, m)
}

func (m *InitialContextSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupRequest(value []byte) (*InitialContextSetupRequest, error) {
	return parseMessageBody[InitialContextSetupRequest](ProcInitialContextSetup, TriggeringInitiatingMessage, initialContextSetupRequestIEs, value)
}

// TS 38.413 §9.2.2.2.
type InitialContextSetupResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceSetup  PDUSessionResourceSetupListCxtRes
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListCxtRes
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupResponseIEs = []ieSpec[InitialContextSetupResponse]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDPDUSessionResourceSetupListCxtRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: IDPDUSessionResourceFailedToSetupListCxtRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *InitialContextSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupResponseIEs, m)
}

func (m *InitialContextSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupResponse(value []byte) (*InitialContextSetupResponse, error) {
	return parseMessageBody[InitialContextSetupResponse](ProcInitialContextSetup, TriggeringSuccessfulOutcome, initialContextSetupResponseIEs, value)
}

// TS 38.413 §9.2.2.3.
type InitialContextSetupFailure struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListCxtFail
	Cause                    *Cause
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var initialContextSetupFailureIEs = []ieSpec[InitialContextSetupFailure]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: IDPDUSessionResourceFailedToSetupListCxtFail, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialContextSetupFailure, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *InitialContextSetupFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *InitialContextSetupFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialContextSetup, initialContextSetupFailureIEs, m)
}

func (m *InitialContextSetupFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcInitialContextSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseInitialContextSetupFailure(value []byte) (*InitialContextSetupFailure, error) {
	return parseMessageBody[InitialContextSetupFailure](ProcInitialContextSetup, TriggeringUnsuccessfulOutcome, initialContextSetupFailureIEs, value)
}
