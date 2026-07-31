// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeModifiedItemBearerModInd ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// dL-GTP-TEID, iE-Extensions OPTIONAL } (extensible) (TS 36.413 §9.2.1.31).
type ERABToBeModifiedItemBearerModInd struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	DLGTPTEID             GTPTEID
	_                     ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.3.8.
type ERABModificationIndication struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ToBeModified            []ERABToBeModifiedItemBearerModInd
	NotToBeModified         []ERABToBeModifiedItemBearerModInd
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABModificationIndicationIEs = []ieSpec[ERABModificationIndication]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABToBeModifiedListBearerModInd, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.ToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeModifiedItemBearerModInd, CriticalityReject, m.ToBeModified)
			}), true
		},
	},
	{
		id: idERABNotToBeModifiedListBearerModInd, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.NotToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			if len(m.NotToBeModified) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABNotToBeModifiedItemBearerModInd, CriticalityReject, m.NotToBeModified)
			}), true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			var (
				err error
				uli UserLocationInformation
			)

			err = perIEDecode(raw, &uli)
			m.UserLocationInformation = &uli

			return err
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *ERABModificationIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABModificationIndication, eRABModificationIndicationIEs, m)
}

func (m *ERABModificationIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABModificationIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABModificationIndication(value []byte) (*ERABModificationIndication, error) {
	return parseMessageBody[ERABModificationIndication](ProcERABModificationIndication, TriggeringInitiatingMessage, eRABModificationIndicationIEs, value)
}

// erabModifyItemBearerModConf ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible).
type erabModifyItemBearerModConf struct {
	_      [0]struct{} `per:"extseq"`
	erabID ERABID
	_      ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.3.9.
type ERABModificationConfirm struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	ModifiedERABs          []ERABID
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var erabModificationConfirmIEs = []ieSpec[ERABModificationConfirm]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABModificationConfirm, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABModificationConfirm, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idERABModifyListBearerModConf, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModificationConfirm, raw []byte, enc per.Encoding) error {
			items, err := decodeItemList[erabModifyItemBearerModConf](per.NewReader(raw), enc, maxnoofERABs)
			if err != nil {
				return err
			}

			m.ModifiedERABs = make([]ERABID, len(items))
			for i, it := range items {
				m.ModifiedERABs[i] = it.erabID
			}

			return nil
		},
		encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if len(m.ModifiedERABs) == 0 {
				return nil, false
			}

			items := make([]erabModifyItemBearerModConf, len(m.ModifiedERABs))
			for i, id := range m.ModifiedERABs {
				items[i] = erabModifyItemBearerModConf{erabID: id}
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABModifyItemBearerModConf, CriticalityIgnore, items)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABModificationConfirm, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *ERABModificationConfirm) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABModificationIndication, erabModificationConfirmIEs, m)
}

func (m *ERABModificationConfirm) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABModificationIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}
