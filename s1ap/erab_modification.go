// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ERABToBeModifiedItemBearerModInd ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// dL-GTP-TEID, iE-Extensions OPTIONAL } (extensible). Names the new downlink S1-U
// endpoint to relocate one E-RAB's GTP tunnel to (TS 36.413 §9.2.1.31).
type ERABToBeModifiedItemBearerModInd struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	DLGTPTEID             GTPTEID
	_                     ieExtensions `per:",skip"`
}

// ERABModificationIndication is the E-RAB MODIFICATION INDICATION message
// (TS 36.413 §9.1.3.8), sent by the eNB to relocate the downlink S1-U endpoint of
// already-established E-RABs. ToBeModified is mandatory; NotToBeModified is optional.
type ERABModificationIndication struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ToBeModified            []ERABToBeModifiedItemBearerModInd
	NotToBeModified         []ERABToBeModifiedItemBearerModInd
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

func (m *ERABModificationIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idERABToBeModifiedListBearerModInd, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeModifiedItemBearerModInd, CriticalityReject, m.ToBeModified)
		})},
	}

	if len(m.NotToBeModified) > 0 {
		fields = append(fields, ieField{id: idERABNotToBeModifiedListBearerModInd, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABNotToBeModifiedItemBearerModInd, CriticalityReject, m.NotToBeModified)
		})})
	}

	if m.UserLocationInformation != nil {
		u := *m.UserLocationInformation
		fields = append(fields, ieField{id: idUserLocationInformation, crit: CriticalityIgnore, val: &u})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU (an eNB-side operation,
// provided for interop testing; the MME only decodes this message).
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

// ParseERABModificationIndication decodes the message from an initiatingMessage
// open-type payload.
func ParseERABModificationIndication(value []byte) (*ERABModificationIndication, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModificationIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &ERABModificationIndication{}

	var seenMME, seenENB, seenToBeModified bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABToBeModifiedListBearerModInd:
			m.ToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](per.NewReader(f.value), enc, maxnoofERABs)
			seenToBeModified = true
		case idERABNotToBeModifiedListBearerModInd:
			m.NotToBeModified, err = decodeItemList[ERABToBeModifiedItemBearerModInd](per.NewReader(f.value), enc, maxnoofERABs)
		case idUserLocationInformation:
			var uli UserLocationInformation

			err = perIEDecode(f.value, &uli)
			m.UserLocationInformation = &uli
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABModificationIndication IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenToBeModified {
		return nil, fmt.Errorf("s1ap: ERABModificationIndication missing mandatory IE")
	}

	return m, nil
}

// erabModifyItemBearerModConf ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible). It confirms one E-RAB whose downlink endpoint was relocated.
type erabModifyItemBearerModConf struct {
	_      [0]struct{} `per:"extseq"`
	erabID ERABID
	_      ieExtensions `per:",skip"`
}

// ERABModificationConfirm is the E-RAB MODIFICATION CONFIRM message
// (TS 36.413 §9.1.3.9), the MME's successful response listing the E-RABs whose
// downlink endpoint it relocated.
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

// Marshal encodes the message as a complete S1AP-PDU.
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
