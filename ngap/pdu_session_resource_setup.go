// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceSetupItemSUReq ::= SEQUENCE { pDUSessionID,
// pDUSessionNAS-PDU OPTIONAL, s-NSSAI, pDUSessionResourceSetupRequestTransfer,
// iE-Extensions OPTIONAL } (extensible). Its own ASN.1 type, so it stays
// distinct from the identically shaped CxtReq item.
type PDUSessionResourceSetupItemSUReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	NASPDU       *NASPDU `per:",optional"`
	SNSSAI       SNSSAI
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListSUReq ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSUReq.
type PDUSessionResourceSetupListSUReq []PDUSessionResourceSetupItemSUReq

// PDUSessionResourceSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceSetupItemSURes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceSetupListSURes ::= SEQUENCE (SIZE(1..maxnoofPDUSessions))
// OF PDUSessionResourceSetupItemSURes.
type PDUSessionResourceSetupListSURes []PDUSessionResourceSetupItemSURes

// PDUSessionResourceFailedToSetupItemSURes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceSetupUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToSetupItemSURes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToSetupListSURes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToSetupItemSURes.
type PDUSessionResourceFailedToSetupListSURes []PDUSessionResourceFailedToSetupItemSURes

// TS 38.413 §9.2.1.1.
type PDUSessionResourceSetupRequest struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	// Optional but reject criticality, unlike the ignore-criticality NAS-PDU of
	// an INITIAL CONTEXT SETUP REQUEST.
	NASPDU                    *NASPDU
	PDUSessionResourceSetup   PDUSessionResourceSetupListSUReq
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate

	messageMeta
}

var pDUSessionResourceSetupRequestIEs = []ieSpec[PDUSessionResourceSetupRequest]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			var v NASPDU

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NASPDU = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return m.NASPDU, true
		},
	},
	{
		id: idPDUSessionResourceSetupListSUReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: idUEAggregateMaximumBitRate, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupRequest, raw []byte, enc per.Encoding) error {
			var v UEAggregateMaximumBitRate

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupRequest) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
}

func (m *PDUSessionResourceSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceSetup, pDUSessionResourceSetupRequestIEs, m)
}

func (m *PDUSessionResourceSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceSetupRequest(value []byte) (*PDUSessionResourceSetupRequest, error) {
	return parseMessageBody[PDUSessionResourceSetupRequest](ProcPDUSessionResourceSetup, TriggeringInitiatingMessage, pDUSessionResourceSetupRequestIEs, value)
}

// TS 38.413 §9.2.1.2.
type PDUSessionResourceSetupResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceSetup  PDUSessionResourceSetupListSURes
	PDUSessionResourceFailed PDUSessionResourceFailedToSetupListSURes
	CriticalityDiagnostics   *CriticalityDiagnostics
	UserLocationInformation  *UserLocationInformation

	messageMeta
}

var pDUSessionResourceSetupResponseIEs = []ieSpec[PDUSessionResourceSetupResponse]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceSetupListSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceSetup)
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceSetup == nil {
				return nil, false
			}

			return m.PDUSessionResourceSetup, true
		},
	},
	{
		id: idPDUSessionResourceFailedToSetupListSURes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceSetupResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceSetupResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *PDUSessionResourceSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceSetup, pDUSessionResourceSetupResponseIEs, m)
}

func (m *PDUSessionResourceSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceSetupResponse(value []byte) (*PDUSessionResourceSetupResponse, error) {
	return parseMessageBody[PDUSessionResourceSetupResponse](ProcPDUSessionResourceSetup, TriggeringSuccessfulOutcome, pDUSessionResourceSetupResponseIEs, value)
}
