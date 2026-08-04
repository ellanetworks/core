// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// PDUSessionResourceModifyItemModInd ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyIndicationTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModInd struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceModifyListModInd ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModInd.
type PDUSessionResourceModifyListModInd []PDUSessionResourceModifyItemModInd

// PDUSessionResourceModifyItemModCfm ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyConfirmTransfer, iE-Extensions OPTIONAL }
// (extensible).
type PDUSessionResourceModifyItemModCfm struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceModifyListModCfm ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceModifyItemModCfm.
type PDUSessionResourceModifyListModCfm []PDUSessionResourceModifyItemModCfm

// PDUSessionResourceFailedToModifyItemModCfm ::= SEQUENCE { pDUSessionID,
// pDUSessionResourceModifyIndicationUnsuccessfulTransfer, iE-Extensions
// OPTIONAL } (extensible).
type PDUSessionResourceFailedToModifyItemModCfm struct {
	_            [0]struct{} `per:"extseq"`
	PDUSessionID PDUSessionID
	Transfer     TransferContainer
	_            ieExtensions `per:",skip"`
}

// PDUSessionResourceFailedToModifyListModCfm ::= SEQUENCE
// (SIZE(1..maxnoofPDUSessions)) OF PDUSessionResourceFailedToModifyItemModCfm.
type PDUSessionResourceFailedToModifyListModCfm []PDUSessionResourceFailedToModifyItemModCfm

// PDUSessionResourceModifyIndicationUnsuccessfulTransfer ::= SEQUENCE { cause,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.20. The first of the
// §9.3.4 transfers modelled: the AMF builds this one itself when it cannot hand
// a session to the SMF, so it cannot stay opaque.
type PDUSessionResourceModifyIndicationUnsuccessfulTransfer struct {
	_     [0]struct{} `per:"extseq"`
	Cause Cause
	_     ieExtensions `per:",skip"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PDUSessionResourceModifyIndicationUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// TS 38.413 §9.2.1.7.
type PDUSessionResourceModifyIndication struct {
	AMFUENGAPID              AMFUENGAPID
	RANUENGAPID              RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModInd
	UserLocationInformation  *UserLocationInformation

	messageMeta
}

var pDUSessionResourceModifyIndicationIEs = []ieSpec[PDUSessionResourceModifyIndication]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idPDUSessionResourceModifyListModInd, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *PDUSessionResourceModifyIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyIndication, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *PDUSessionResourceModifyIndication) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *PDUSessionResourceModifyIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyIndicationIEs, m)
}

func (m *PDUSessionResourceModifyIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcPDUSessionResourceModifyIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyIndication(value []byte) (*PDUSessionResourceModifyIndication, error) {
	return parseMessageBody[PDUSessionResourceModifyIndication](ProcPDUSessionResourceModifyIndication, TriggeringInitiatingMessage, pDUSessionResourceModifyIndicationIEs, value)
}

// TS 38.413 §9.2.1.8.
type PDUSessionResourceModifyConfirm struct {
	AMFUENGAPID              *AMFUENGAPID
	RANUENGAPID              *RANUENGAPID
	PDUSessionResourceModify PDUSessionResourceModifyListModCfm
	PDUSessionResourceFailed PDUSessionResourceFailedToModifyListModCfm
	CriticalityDiagnostics   *CriticalityDiagnostics

	messageMeta
}

var pDUSessionResourceModifyConfirmIEs = []ieSpec[PDUSessionResourceModifyConfirm]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyConfirm, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyConfirm, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idPDUSessionResourceModifyListModCfm, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyConfirm, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceModify)
		},
		encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.PDUSessionResourceModify == nil {
				return nil, false
			}

			return m.PDUSessionResourceModify, true
		},
	},
	{
		id: idPDUSessionResourceFailedToModifyListModCfm, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyConfirm, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.PDUSessionResourceFailed)
		},
		encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.PDUSessionResourceFailed == nil {
				return nil, false
			}

			return m.PDUSessionResourceFailed, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *PDUSessionResourceModifyConfirm, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *PDUSessionResourceModifyConfirm) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *PDUSessionResourceModifyConfirm) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcPDUSessionResourceModifyIndication, pDUSessionResourceModifyConfirmIEs, m)
}

func (m *PDUSessionResourceModifyConfirm) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcPDUSessionResourceModifyIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParsePDUSessionResourceModifyConfirm(value []byte) (*PDUSessionResourceModifyConfirm, error) {
	return parseMessageBody[PDUSessionResourceModifyConfirm](ProcPDUSessionResourceModifyIndication, TriggeringSuccessfulOutcome, pDUSessionResourceModifyConfirmIEs, value)
}
