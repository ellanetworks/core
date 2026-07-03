// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nrppa provides a labeled, JSON-serializable decoder view of NRPPa
// PDUs (TS 38.455 E-CID Measurement Initiation procedures) for the radio-event
// inspector. It wraps the internal/nrppa codec and renders enums as
// utils.EnumField so the UI shows "Label (value)", matching the NAS decoder.
package nrppa

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	corenrppa "github.com/ellanetworks/core/internal/nrppa"
)

// Message is the decoder view of a parsed NRPPa PDU.
type Message struct {
	Kind        utils.EnumField[int] `json:"kind"`
	Request     *Request             `json:"request,omitempty"`
	Response    *Response            `json:"response,omitempty"`
	Failure     *Failure             `json:"failure,omitempty"`
	Termination *Termination         `json:"termination,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// Request is a decoded E-CIDMeasurementInitiationRequest.
type Request struct {
	LMFUEMeasurementID    int64                  `json:"lmf_ue_measurement_id"`
	ReportCharacteristics utils.EnumField[int]   `json:"report_characteristics"`
	MeasurementQuantities []utils.EnumField[int] `json:"measurement_quantities,omitempty"`
}

// Response is a decoded E-CIDMeasurementInitiationResponse.
type Response struct {
	LMFUEMeasurementID int64   `json:"lmf_ue_measurement_id"`
	RANUEMeasurementID int64   `json:"ran_ue_measurement_id"`
	CellPortionID      *int64  `json:"cell_portion_id,omitempty"`
	Result             *Result `json:"result,omitempty"`
}

// Failure is a decoded E-CIDMeasurementInitiationFailure.
type Failure struct {
	LMFUEMeasurementID int64 `json:"lmf_ue_measurement_id"`
	Cause              Cause `json:"cause"`
}

// Termination is a decoded E-CIDMeasurementTerminationCommand.
type Termination struct {
	LMFUEMeasurementID int64 `json:"lmf_ue_measurement_id"`
	RANUEMeasurementID int64 `json:"ran_ue_measurement_id"`
}

// Cause is a decoded NRPPa Cause.
type Cause struct {
	Group utils.EnumField[int] `json:"group"`
	Value int64                `json:"value"`
}

// Result is the gNB-supplied E-CID measurement result.
type Result struct {
	ServingCell        ServingCell `json:"serving_cell"`
	ServingCellTAC     string      `json:"serving_cell_tac"`
	APPosition         *APPosition `json:"ap_position,omitempty"`
	TimingAdvanceType1 *int64      `json:"timing_advance_type1,omitempty"`
	TimingAdvanceType2 *int64      `json:"timing_advance_type2,omitempty"`
	NRTimingAdvance    *int64      `json:"nr_timing_advance,omitempty"`
	UERxTxTimeDiff     *int64      `json:"ue_rx_tx_time_diff,omitempty"`
	AoA                *AoA        `json:"angle_of_arrival,omitempty"`
	SSRSRP             []RSItem    `json:"ss_rsrp,omitempty"`
	SSRSRQ             []RSItem    `json:"ss_rsrq,omitempty"`
	CSIRSRP            []CSIRSItem `json:"csi_rsrp,omitempty"`
	CSIRSRQ            []CSIRSItem `json:"csi_rsrq,omitempty"`
}

// ServingCell is the decoded NG-RAN CGI.
type ServingCell struct {
	PLMNIdentity   string  `json:"plmn_identity"`
	NRCellIdentity *string `json:"nr_cell_identity,omitempty"`
	EUTRACellID    *string `json:"eutra_cell_id,omitempty"`
}

// APPosition is the decoded NG-RAN Access Point Position.
type APPosition struct {
	LatitudeDegrees      float64 `json:"latitude_degrees"`
	LongitudeDegrees     float64 `json:"longitude_degrees"`
	Altitude             int64   `json:"altitude"`
	UncertaintySemiMajor int64   `json:"uncertainty_semi_major"`
	UncertaintySemiMinor int64   `json:"uncertainty_semi_minor"`
	Confidence           int64   `json:"confidence"`
}

// AoA is the decoded UL Angle of Arrival (azimuth, optional zenith), degrees.
type AoA struct {
	AzimuthDegrees float64  `json:"azimuth_degrees"`
	ZenithDegrees  *float64 `json:"zenith_degrees,omitempty"`
}

// RSItem is a per-cell SS-RSRP/SS-RSRQ report (raw on-wire report value).
type RSItem struct {
	NRPCI int64 `json:"nr_pci"`
	Value int64 `json:"value"`
}

// CSIRSItem is a per-resource CSI-RSRP/CSI-RSRQ report (raw report value).
type CSIRSItem struct {
	NRPCI      int64 `json:"nr_pci"`
	CSIRSIndex int64 `json:"csi_rs_index"`
	Value      int64 `json:"value"`
}

// Decode parses a raw NRPPa PDU into the labeled view. It applies the same
// "retry with the first byte stripped" fallback used elsewhere. On decode
// failure it returns a Message with only Error set.
func Decode(raw []byte) *Message {
	parsed, err := corenrppa.ParsePDU(raw)
	if err != nil && len(raw) > 1 {
		parsed, err = corenrppa.ParsePDU(raw[1:])
	}

	if err != nil {
		return &Message{Error: err.Error()}
	}

	msg := &Message{Kind: kindEnum(parsed.Kind)}

	switch {
	case parsed.Request != nil:
		msg.Request = mapRequest(parsed.Request)
	case parsed.Response != nil:
		msg.Response = mapResponse(parsed.Response)
	case parsed.Failure != nil:
		msg.Failure = mapFailure(parsed.Failure)
	case parsed.Termination != nil:
		msg.Termination = &Termination{
			LMFUEMeasurementID: parsed.Termination.LMFUEMeasurementID,
			RANUEMeasurementID: parsed.Termination.RANUEMeasurementID,
		}
	}

	return msg
}

func mapRequest(r *corenrppa.ECIDRequest) *Request {
	out := &Request{
		LMFUEMeasurementID:    r.LMFUEMeasurementID,
		ReportCharacteristics: reportCharacteristicsEnum(r.ReportCharacteristics),
	}

	for _, q := range r.MeasurementQuantities {
		out.MeasurementQuantities = append(out.MeasurementQuantities, measurementQuantityEnum(q))
	}

	return out
}

func mapResponse(r *corenrppa.ECIDResponse) *Response {
	out := &Response{
		LMFUEMeasurementID: r.LMFUEMeasurementID,
		RANUEMeasurementID: r.RANUEMeasurementID,
		CellPortionID:      r.CellPortionID,
	}

	if r.Result != nil {
		out.Result = mapResult(r.Result)
	}

	return out
}

func mapFailure(f *corenrppa.ECIDFailure) *Failure {
	return &Failure{
		LMFUEMeasurementID: f.LMFUEMeasurementID,
		Cause: Cause{
			Group: causeGroupEnum(f.Cause.Group),
			Value: f.Cause.Value,
		},
	}
}

func mapResult(r *corenrppa.ECIDResult) *Result {
	out := &Result{
		ServingCell:        mapServingCell(r.ServingCell),
		ServingCellTAC:     hex.EncodeToString(r.ServingCellTAC),
		TimingAdvanceType1: r.TimingAdvanceType1,
		TimingAdvanceType2: r.TimingAdvanceType2,
		NRTimingAdvance:    r.NRTimingAdvance,
		UERxTxTimeDiff:     r.UERxTxTimeDiff,
	}

	if ap := r.APPosition; ap != nil {
		out.APPosition = &APPosition{
			LatitudeDegrees:      ap.LatitudeDegrees,
			LongitudeDegrees:     ap.LongitudeDegrees,
			Altitude:             ap.Altitude,
			UncertaintySemiMajor: ap.UncertaintySemiMajor,
			UncertaintySemiMinor: ap.UncertaintySemiMinor,
			Confidence:           ap.Confidence,
		}
	}

	if r.AoA != nil {
		out.AoA = &AoA{AzimuthDegrees: r.AoA.AzimuthDegrees, ZenithDegrees: r.AoA.ZenithDegrees}
	}

	if r.ResultSSRSRP != nil {
		for _, it := range r.ResultSSRSRP.Items {
			out.SSRSRP = append(out.SSRSRP, RSItem{NRPCI: it.NRPCI, Value: it.Value})
		}
	}

	if r.ResultSSRSRQ != nil {
		for _, it := range r.ResultSSRSRQ.Items {
			out.SSRSRQ = append(out.SSRSRQ, RSItem{NRPCI: it.NRPCI, Value: it.Value})
		}
	}

	if r.ResultCSIRSRP != nil {
		for _, it := range r.ResultCSIRSRP.Items {
			out.CSIRSRP = append(out.CSIRSRP, CSIRSItem{NRPCI: it.NRPCI, CSIRSIndex: it.CSIRSIndex, Value: it.Value})
		}
	}

	if r.ResultCSIRSRQ != nil {
		for _, it := range r.ResultCSIRSRQ.Items {
			out.CSIRSRQ = append(out.CSIRSRQ, CSIRSItem{NRPCI: it.NRPCI, CSIRSIndex: it.CSIRSIndex, Value: it.Value})
		}
	}

	return out
}

func mapServingCell(sc corenrppa.ServingCell) ServingCell {
	out := ServingCell{PLMNIdentity: hex.EncodeToString(sc.PLMNIdentity)}

	if sc.NRCellIdentity != nil {
		s := fmt.Sprintf("%09x", *sc.NRCellIdentity)
		out.NRCellIdentity = &s
	}

	if sc.EUTRACellID != nil {
		s := fmt.Sprintf("%07x", *sc.EUTRACellID)
		out.EUTRACellID = &s
	}

	return out
}

// --- enum label helpers ---

func kindEnum(k corenrppa.MessageKind) utils.EnumField[int] {
	label := map[corenrppa.MessageKind]string{
		corenrppa.KindECIDMeasurementInitiationRequest:  "E-CID Measurement Initiation Request",
		corenrppa.KindECIDMeasurementInitiationResponse: "E-CID Measurement Initiation Response",
		corenrppa.KindECIDMeasurementInitiationFailure:  "E-CID Measurement Initiation Failure",
		corenrppa.KindECIDMeasurementTerminationCommand: "E-CID Measurement Termination Command",
	}[k]

	return utils.MakeEnum(int(k), label, label == "")
}

func reportCharacteristicsEnum(v int) utils.EnumField[int] {
	switch v {
	case 0:
		return utils.MakeEnum(v, "onDemand", false)
	case 1:
		return utils.MakeEnum(v, "periodic", false)
	default:
		return utils.MakeEnum(v, "", true)
	}
}

func measurementQuantityEnum(q corenrppa.MeasurementQuantityValue) utils.EnumField[int] {
	label := map[corenrppa.MeasurementQuantityValue]string{
		corenrppa.MeasCellID:             "cell-ID",
		corenrppa.MeasAngleOfArrival:     "angleOfArrival",
		corenrppa.MeasTimingAdvanceType1: "timingAdvanceType1",
		corenrppa.MeasTimingAdvanceType2: "timingAdvanceType2",
		corenrppa.MeasRSRP:               "rSRP",
		corenrppa.MeasRSRQ:               "rSRQ",
		corenrppa.MeasSSRSRP:             "ss-RSRP",
		corenrppa.MeasSSRSRQ:             "ss-RSRQ",
		corenrppa.MeasCSIRSRP:            "csi-RSRP",
		corenrppa.MeasCSIRSRQ:            "csi-RSRQ",
		corenrppa.MeasAngleOfArrivalNR:   "angleOfArrivalNR",
		corenrppa.MeasTimingAdvanceNR:    "timingAdvanceNR",
		corenrppa.MeasUERxTxTimeDiff:     "uE-Rx-Tx-Time-Diff",
	}[q]

	return utils.MakeEnum(int(q), label, label == "")
}

func causeGroupEnum(g corenrppa.CauseGroup) utils.EnumField[int] {
	switch g {
	case corenrppa.CauseGroupRadioNetwork:
		return utils.MakeEnum(int(g), "radioNetwork", false)
	case corenrppa.CauseGroupProtocol:
		return utils.MakeEnum(int(g), "protocol", false)
	case corenrppa.CauseGroupMisc:
		return utils.MakeEnum(int(g), "misc", false)
	case corenrppa.CauseGroupChoiceExtension:
		return utils.MakeEnum(int(g), "choice-Extension", false)
	default:
		return utils.MakeEnum(int(g), "", true)
	}
}
