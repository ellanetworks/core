// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// ERABToBeModifiedItemBearerModInd ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// dL-GTP-TEID, iE-Extensions OPTIONAL } (extensible). Names the new downlink S1-U
// endpoint to relocate one E-RAB's GTP tunnel to (TS 36.413 §9.2.1.31).
type ERABToBeModifiedItemBearerModInd struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	DLGTPTEID             GTPTEID
}

func (it ERABToBeModifiedItemBearerModInd) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	return it.DLGTPTEID.encode(w)
}

func decodeERABToBeModifiedItemBearerModInd(r *aper.Reader) (ERABToBeModifiedItemBearerModInd, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABToBeModifiedItemBearerModInd{}, err
	}

	var it ERABToBeModifiedItemBearerModInd

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.DLGTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
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

func (m *ERABModificationIndication) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idERABToBeModifiedListBearerModInd, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABToBeModifiedItemBearerModInd, CriticalityReject, encoderList(m.ToBeModified))
		}},
	}

	if len(m.NotToBeModified) > 0 {
		fields = append(fields, ieField{id: idERABNotToBeModifiedListBearerModInd, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABNotToBeModifiedItemBearerModInd, CriticalityReject, encoderList(m.NotToBeModified))
		}})
	}

	if m.UserLocationInformation != nil {
		u := *m.UserLocationInformation
		fields = append(fields, ieField{id: idUserLocationInformation, crit: CriticalityIgnore, enc: u.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU (an eNB-side operation,
// provided for interop testing; the MME only decodes this message).
func (m *ERABModificationIndication) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABModificationIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABModificationIndication decodes the message from an initiatingMessage
// open-type payload.
func ParseERABModificationIndication(value []byte) (*ERABModificationIndication, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABModificationIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ERABModificationIndication{}

	var seenMME, seenENB, seenToBeModified bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABToBeModifiedListBearerModInd:
			m.ToBeModified, err = decodeItemList(sub, maxnoofERABs, decodeERABToBeModifiedItemBearerModInd)
			seenToBeModified = true
		case idERABNotToBeModifiedListBearerModInd:
			m.NotToBeModified, err = decodeItemList(sub, maxnoofERABs, decodeERABToBeModifiedItemBearerModInd)
		case idUserLocationInformation:
			var uli UserLocationInformation

			uli, err = decodeUserLocationInformation(sub)
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
	erabID ERABID
}

func (it erabModifyItemBearerModConf) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	return it.erabID.encode(w)
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

func (m *ERABModificationConfirm) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
	}

	if len(m.ModifiedERABs) > 0 {
		items := make([]erabModifyItemBearerModConf, len(m.ModifiedERABs))
		for i, id := range m.ModifiedERABs {
			items[i] = erabModifyItemBearerModConf{erabID: id}
		}

		fields = append(fields, ieField{
			id:   idERABModifyListBearerModConf,
			crit: CriticalityIgnore,
			enc: func(w *aper.Writer) error {
				return encodeSingleContainerList(w, maxnoofERABs, idERABModifyItemBearerModConf, CriticalityIgnore, encoderList(items))
			},
		})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABModificationConfirm) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABModificationIndication,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}
