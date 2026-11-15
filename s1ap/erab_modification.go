// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABToBeModifiedItemBearerModInd ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// dL-GTP-TEID, iE-Extensions OPTIONAL } (extensible) (TS 36.413 §9.2.1.31).
type ERABToBeModifiedItemBearerModInd struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	DLGTPTEID             GTPTEID
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
	containerCrit         `per:"-"`
}

// TS 36.413 §9.1.3.8.
type ERABModificationIndication struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ToBeModified            ERABToBeModifiedListBearerModInd
	NotToBeModified         ERABNotToBeModifiedListBearerModInd
	UserLocationInformation *UserLocationInformation
	TunnelInformationForBBF *TunnelInformation

	messageMeta
}

var eRABModificationIndicationIEs = []ieSpec[ERABModificationIndication]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDERABToBeModifiedListBearerModInd, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](r, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModInd, CriticalityReject)

			return err
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeModifiedItemBearerModInd, CriticalityReject, m.ToBeModified), true
		},
	},
	{
		ID: IDERABNotToBeModifiedListBearerModInd, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.NotToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](r, enc, maxnoofERABs, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject)

			return err
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			if len(m.NotToBeModified) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABNotToBeModifiedItemBearerModInd, CriticalityReject, m.NotToBeModified), true
		},
	},
	{
		ID: IDTunnelInformationForBBF, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TunnelInformationForBBF)
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
			return optionalIE(m.TunnelInformationForBBF)
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationIndication, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *ERABModificationIndication) (per.Marshaler, bool) {
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

func DecodeERABModificationIndication(value []byte) (*ERABModificationIndication, error) {
	return decodeMessageBody[ERABModificationIndication](ProcERABModificationIndication, eRABModificationIndicationIEs, value)
}

// erabModifyItemBearerModConf ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible).
type erabModifyItemBearerModConf struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	erabID       ERABID
	iEExtensions extensionMeta `per:",exttable"`
}

// TS 36.413 §9.1.3.9.
type ERABModificationConfirm struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	ModifiedERABs          []ERABID
	FailedToModify         []ERABItem
	ToBeReleased           []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var erabModificationConfirmIEs = []ieSpec[ERABModificationConfirm]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABModifyListBearerModConf, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			items, err := decodeItemList[erabModifyItemBearerModConf](r, enc, maxnoofERABs, IDERABModifyItemBearerModConf, CriticalityIgnore)
			if err != nil {
				return err
			}

			m.ModifiedERABs = make([]ERABID, len(items))
			for i, it := range items {
				m.ModifiedERABs[i] = it.erabID
			}

			return nil
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if len(m.ModifiedERABs) == 0 {
				return nil, false
			}

			items := make([]erabModifyItemBearerModConf, len(m.ModifiedERABs))
			for i, id := range m.ModifiedERABs {
				items[i] = erabModifyItemBearerModConf{erabID: id}
			}

			return containerList(maxnoofERABs, IDERABModifyItemBearerModConf, CriticalityIgnore, items), true
		},
	},
	{
		ID: IDERABFailedToModifyListBearerModConf, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			var err error

			m.FailedToModify, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if len(m.FailedToModify) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.FailedToModify), true
		},
	},
	{
		ID: IDERABToBeReleasedListBearerModConf, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ToBeReleased, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
			if len(m.ToBeReleased) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ToBeReleased), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABModificationConfirm, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *ERABModificationConfirm) (per.Marshaler, bool) {
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

func ParseERABModificationConfirm(value []byte) (*ERABModificationConfirm, error) {
	return parseMessageBody[ERABModificationConfirm](ProcERABModificationIndication, TriggeringSuccessfulOutcome, erabModificationConfirmIEs, value)
}

func DecodeERABModificationConfirm(value []byte) (*ERABModificationConfirm, error) {
	return decodeMessageBody[ERABModificationConfirm](ProcERABModificationIndication, erabModificationConfirmIEs, value)
}
