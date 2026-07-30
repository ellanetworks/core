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

	unmodeledIEs
}

var eRABModificationIndicationIEs = []ieSpec[ERABModificationIndication]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABModificationIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABToBeModifiedListBearerModInd, presence: PresenceMandatory, crit: CriticalityReject,
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
		id: idERABNotToBeModifiedListBearerModInd, presence: PresenceOptional, crit: CriticalityReject,
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
		id: idUserLocationInformation, presence: PresenceOptional, crit: CriticalityIgnore,
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
	return encodeMessageBody(w, enc, eRABModificationIndicationIEs, m)
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
	return parseMessageBody[ERABModificationIndication](ProcERABModificationIndication, eRABModificationIndicationIEs, value)
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
	MMEUES1APID   MMEUES1APID
	ENBUES1APID   ENBUES1APID
	ModifiedERABs []ERABID

	unmodeledIEs
}

func (m *ERABModificationConfirm) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
	}

	if len(m.ModifiedERABs) > 0 {
		items := make([]erabModifyItemBearerModConf, len(m.ModifiedERABs))
		for i, id := range m.ModifiedERABs {
			items[i] = erabModifyItemBearerModConf{erabID: id}
		}

		fields = append(fields, ieField{
			id:   idERABModifyListBearerModConf,
			crit: CriticalityIgnore,
			val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABModifyItemBearerModConf, CriticalityIgnore, items)
			}),
		})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
