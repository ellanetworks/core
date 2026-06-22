// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// ERABToBeSetupItemBearerSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU, iE-Extensions OPTIONAL }
// (extensible). The NAS-PDU is mandatory:
// the E-RAB Setup carries the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST for an
// additional PDN connection (TS 36.413 §9.1.3.1).
type ERABToBeSetupItemBearerSUReq struct {
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU
}

func (it ERABToBeSetupItemBearerSUReq) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.QoS.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	if err := it.GTPTEID.encode(w); err != nil {
		return err
	}

	return it.NASPDU.encode(w)
}

func decodeERABToBeSetupItemBearerSUReq(value []byte) (ERABToBeSetupItemBearerSUReq, error) {
	r := aper.NewReader(value)

	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABToBeSetupItemBearerSUReq{}, err
	}

	var it ERABToBeSetupItemBearerSUReq

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.QoS, err = decodeERABLevelQoSParameters(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if it.NASPDU, err = decodeNASPDU(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

// ERABSetupItemBearerSURes has the same structure as ERABSetupItemCtxtSURes
// (e-RAB-ID, transportLayerAddress, gTP-TEID): the eNB endpoint the UPF sends
// downlink traffic to (TS 36.413 §9.1.3.2). The two decode identically.
type ERABSetupItemBearerSURes = ERABSetupItemCtxtSURes

// ERABSetupRequest is the E-RAB SETUP REQUEST message (TS 36.413 §9.1.3.1), sent
// by the MME to add one or more E-RABs (and their default bearers) to an
// established UE context — the radio leg of an additional PDN connection.
type ERABSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemBearerSUReq

	unmodeledIEs
}

func (m *ERABSetupRequest) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, enc: ambr.encode})
	}

	fields = append(fields, ieField{id: idERABToBeSetupListBearerSUReq, crit: CriticalityReject, enc: func(w *aper.Writer) error {
		return encodeSingleContainerList(w, maxnoofERABs, idERABToBeSetupItemBearerSUReq, CriticalityReject, encoderList(m.ERABToBeSetup))
	}})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABSetupRequest) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABSetupRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseERABSetupRequest(value []byte) (*ERABSetupRequest, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABSetupRequest preamble: %w", err)
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

	m := &ERABSetupRequest{}

	var seenMME, seenENB, seenERAB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			ambr, err = decodeUEAggregateMaximumBitRate(sub)
			m.UEAggregateMaximumBitRate = &ambr
		case idERABToBeSetupListBearerSUReq:
			m.ERABToBeSetup, err = decodeERABToBeSetupBearerList(sub)
			seenERAB = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABSetupRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenERAB {
		return nil, fmt.Errorf("s1ap: ERABSetupRequest missing mandatory IE")
	}

	return m, nil
}

func decodeERABToBeSetupBearerList(r *aper.Reader) ([]ERABToBeSetupItemBearerSUReq, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABToBeSetupItemBearerSUReq)
}

// ERABSetupResponse is the E-RAB SETUP RESPONSE message (TS 36.413 §9.1.3.2),
// sent by the eNB once the E-RAB(s) are set up. ERABSetup carries the eNB S1-U
// endpoint for each established E-RAB; ERABFailedToSetup lists those rejected.
type ERABSetupResponse struct {
	MMEUES1APID            MMEUES1APID
	ENBUES1APID            ENBUES1APID
	ERABSetup              []ERABSetupItemBearerSURes
	ERABFailedToSetup      []ERABItem
	CriticalityDiagnostics *CriticalityDiagnostics

	unmodeledIEs
}

func (m *ERABSetupResponse) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
	}

	if len(m.ERABSetup) > 0 {
		fields = append(fields, ieField{id: idERABSetupListBearerSURes, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABSetupItemBearerSURes, CriticalityIgnore, encoderList(m.ERABSetup))
		}})
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListBearerSURes, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABItem, CriticalityIgnore, encoderList(m.ERABFailedToSetup))
		}})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, enc: d.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABSetupResponse) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABSetupResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABSetupResponse(value []byte) (*ERABSetupResponse, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABSetupResponse preamble: %w", err)
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

	m := &ERABSetupResponse{}

	var seenMME, seenENB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idERABSetupListBearerSURes:
			m.ERABSetup, err = decodeERABSetupList(sub)
		case idERABFailedToSetupListBearerSURes:
			m.ERABFailedToSetup, err = decodeERABItemList(sub)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			cd, err = decodeCriticalityDiagnostics(sub)
			m.CriticalityDiagnostics = &cd
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABSetupResponse IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB {
		return nil, fmt.Errorf("s1ap: ERABSetupResponse missing mandatory IE")
	}

	return m, nil
}
