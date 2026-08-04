// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceModifyItemModReq ::= SEQUENCE { pDUSessionID, nAS-PDU
// OPTIONAL, pDUSessionResourceModifyRequestTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModReq struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	NASPDU       *NASPDU `per:",optional"`
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceModifyListModReq ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModReq.
type PDUSessionResourceModifyListModReq []PDUSessionResourceModifyItemModReq

// PDUSessionResourceModifyItemModRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyResponseTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModRes.
type PDUSessionResourceModifyListModRes []PDUSessionResourceModifyItemModRes

// PDUSessionResourceFailedToModifyItemModRes ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyUnsuccessfulTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceFailedToModifyItemModRes struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToModifyListModRes ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToModifyItemModRes.
type PDUSessionResourceFailedToModifyListModRes []PDUSessionResourceFailedToModifyItemModRes

// TS 38.413 §9.2.1.5.
type PDUSessionResourceModifyRequest struct {
	AMFUENGAPID              AMFUENGAPID
	RANUENGAPID              RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModReq

	messageMeta
}

var pDUSessionResourceModifyRequestIEs = []ieSpec[PDUSessionResourceModifyRequest]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idPDUSessionResourceModifyListModReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyRequest) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
}

func (m *PDUSessionResourceModifyRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModify, pDUSessionResourceModifyRequestIEs, m)
}

func (m *PDUSessionResourceModifyRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyRequest(value []byte) (*PDUSessionResourceModifyRequest, error) {
	return parseMessageBody[PDUSessionResourceModifyRequest](ProcPDUSessionResourceModify, TriggeringInitiatingMessage, pDUSessionResourceModifyRequestIEs, value)
}

// TS 38.413 §9.2.1.6.
type PDUSessionResourceModifyResponse struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModRes
	PDUSessionResourceFailed PDUSessionResourceFailedToModifyListModRes
	UserLocationInformation  *UserLocationInformation
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var pDUSessionResourceModifyResponseIEs = []ieSpec[PDUSessionResourceModifyResponse]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceModifyListModRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		id: idPDUSessionResourceFailedToModifyListModRes, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceModifyResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PDUSessionResourceModifyResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModify, pDUSessionResourceModifyResponseIEs, m)
}

func (m *PDUSessionResourceModifyResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceModify,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyResponse(value []byte) (*PDUSessionResourceModifyResponse, error) {
	return parseMessageBody[PDUSessionResourceModifyResponse](ProcPDUSessionResourceModify, TriggeringSuccessfulOutcome, pDUSessionResourceModifyResponseIEs, value)
}
