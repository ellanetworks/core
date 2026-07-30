// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ERABToBeSetupItemBearerSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU, iE-Extensions OPTIONAL }
// (extensible). The NAS-PDU is mandatory:
// the E-RAB Setup carries the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST for an
// additional PDN connection (TS 36.413).
type ERABToBeSetupItemBearerSUReq struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU
	_                     ieExtensions `per:",skip"`
}

// ERABSetupItemBearerSURes has the same structure as ERABSetupItemCtxtSURes
// (e-RAB-ID, transportLayerAddress, gTP-TEID): the eNB endpoint the UPF sends
// downlink traffic to (TS 36.413). The two decode identically.
type ERABSetupItemBearerSURes = ERABSetupItemCtxtSURes

// ERABSetupRequest is the E-RAB SETUP REQUEST message (TS 36.413), sent
// by the MME to add one or more E-RABs (and their default bearers) to an
// established UE context — the radio leg of an additional PDN connection.
type ERABSetupRequest struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeSetup             []ERABToBeSetupItemBearerSUReq

	unmodeledIEs
}

func (m *ERABSetupRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
	}

	if m.UEAggregateMaximumBitRate != nil {
		ambr := *m.UEAggregateMaximumBitRate
		fields = append(fields, ieField{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, val: &ambr})
	}

	fields = append(fields, ieField{id: idERABToBeSetupListBearerSUReq, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
		return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemBearerSUReq, CriticalityReject, m.ERABToBeSetup)
	})})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABSetupRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABSetupRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseERABSetupRequest(value []byte) (*ERABSetupRequest, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABSetupRequest preamble: %w", err)
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

	m := &ERABSetupRequest{}

	var seenMME, seenENB, seenERAB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idUEAggregateMaximumBitrate:
			var ambr UEAggregateMaximumBitRate

			err = perIEDecode(f.value, &ambr)
			m.UEAggregateMaximumBitRate = &ambr
		case idERABToBeSetupListBearerSUReq:
			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemBearerSUReq](per.NewReader(f.value), enc, maxnoofERABs)
			seenERAB = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABSetupRequest IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcERABSetup,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idERABToBeSetupListBearerSUReq, CriticalityReject, seenERAB},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// ERABSetupResponse is the E-RAB SETUP RESPONSE message (TS 36.413),
// sent by the eNB once the E-RAB(s) are set up. ERABSetup carries the eNB S1-U
// endpoint for each established E-RAB; ERABFailedToSetup lists those rejected.
type ERABSetupResponse struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ERABSetup               []ERABSetupItemBearerSURes
	ERABFailedToSetup       []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

func (m *ERABSetupResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
	}

	if len(m.ERABSetup) > 0 {
		fields = append(fields, ieField{id: idERABSetupListBearerSURes, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABSetupItemBearerSURes, CriticalityIgnore, m.ERABSetup)
		})})
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListBearerSURes, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
		})})
	}

	if m.CriticalityDiagnostics != nil {
		d := *m.CriticalityDiagnostics
		fields = append(fields, ieField{id: idCriticalityDiagnostics, crit: CriticalityIgnore, val: &d})
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

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ERABSetupResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABSetup,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseERABSetupResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABSetupResponse(value []byte) (*ERABSetupResponse, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ERABSetupResponse preamble: %w", err)
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

	m := &ERABSetupResponse{}

	var seenMME, seenENB bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABSetupListBearerSURes:
			m.ERABSetup, err = decodeItemList[ERABSetupItemBearerSURes](per.NewReader(f.value), enc, maxnoofERABs)
		case idERABFailedToSetupListBearerSURes:
			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		case idCriticalityDiagnostics:
			var cd CriticalityDiagnostics

			err = perIEDecode(f.value, &cd)
			m.CriticalityDiagnostics = &cd
		case idUserLocationInformation:
			var uli UserLocationInformation

			err = perIEDecode(f.value, &uli)
			m.UserLocationInformation = &uli
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ERABSetupResponse IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcERABSetup,
		ieCheck{idMMEUES1APID, CriticalityIgnore, seenMME},
		ieCheck{idENBUES1APID, CriticalityIgnore, seenENB},
	); err != nil {
		return nil, err
	}

	return m, nil
}
