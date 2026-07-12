// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
)

// LPPaPDU is the decoder view of an LPPa PDU carried inside an S1AP UE-associated
// LPPa transport message. It mirrors the NASPDU wrapper: the raw octet string as
// hex, plus the decoded E-CID content (TS 36.455).
type LPPaPDU struct {
	Protocol string       `json:"protocol"`
	RawHex   string       `json:"raw_hex"`
	Decoded  *LPPaMessage `json:"decoded,omitempty"`
}

type LPPaMessage struct {
	Kind                 string          `json:"kind"`
	ESMLCUEMeasurementID int64           `json:"esmlc_ue_measurement_id,omitempty"`
	ENBUEMeasurementID   int64           `json:"enb_ue_measurement_id,omitempty"`
	Result               *LPPaECIDResult `json:"result,omitempty"`
}

type LPPaECIDResult struct {
	ServingCellID  string `json:"serving_cell_id,omitempty"` // 28-bit E-UTRA cell identity, hex
	TimingAdvance  *int64 `json:"timing_advance,omitempty"`
	AngleOfArrival *int64 `json:"angle_of_arrival,omitempty"`
	RSRPCells      int    `json:"rsrp_cells,omitempty"`
	RSRQCells      int    `json:"rsrq_cells,omitempty"`
	HasAPPosition  bool   `json:"has_ap_position,omitempty"`
}

func decodeLPPaPDU(raw []byte) LPPaPDU {
	w := LPPaPDU{Protocol: "LPPa", RawHex: hex.EncodeToString(raw)}

	d, err := lppa.ParsePDU(raw)
	if err != nil {
		return w
	}

	w.Decoded = mapLPPaMessage(d)

	return w
}

func mapLPPaMessage(d *lppa.ParsedPDU) *LPPaMessage {
	m := &LPPaMessage{Kind: lppaKindName(d.Kind)}

	switch {
	case d.Request != nil:
		m.ESMLCUEMeasurementID = d.Request.ESMLCUEMeasurementID
	case d.Response != nil:
		m.ESMLCUEMeasurementID = d.Response.ESMLCUEMeasurementID
		m.ENBUEMeasurementID = d.Response.ENBUEMeasurementID
		m.Result = mapLPPaResult(d.Response.Result)
	case d.Failure != nil:
		m.ESMLCUEMeasurementID = d.Failure.ESMLCUEMeasurementID
	case d.FailureIndication != nil:
		m.ESMLCUEMeasurementID = d.FailureIndication.ESMLCUEMeasurementID
		m.ENBUEMeasurementID = d.FailureIndication.ENBUEMeasurementID
	case d.Termination != nil:
		m.ESMLCUEMeasurementID = d.Termination.ESMLCUEMeasurementID
		m.ENBUEMeasurementID = d.Termination.ENBUEMeasurementID
	}

	return m
}

func mapLPPaResult(r *lppa.ECIDResult) *LPPaECIDResult {
	if r == nil {
		return nil
	}

	return &LPPaECIDResult{
		ServingCellID:  fmt.Sprintf("%07x", r.ServingCell.EUTRACellID),
		TimingAdvance:  r.TimingAdvanceType1,
		AngleOfArrival: r.AngleOfArrival,
		RSRPCells:      len(r.RSRP),
		RSRQCells:      len(r.RSRQ),
		HasAPPosition:  r.APPosition != nil,
	}
}

func lppaKindName(k lppa.MessageKind) string {
	switch k {
	case lppa.KindECIDMeasurementInitiationRequest:
		return "E-CIDMeasurementInitiationRequest"
	case lppa.KindECIDMeasurementInitiationResponse:
		return "E-CIDMeasurementInitiationResponse"
	case lppa.KindECIDMeasurementInitiationFailure:
		return "E-CIDMeasurementInitiationFailure"
	case lppa.KindECIDMeasurementTerminationCommand:
		return "E-CIDMeasurementTerminationCommand"
	case lppa.KindECIDMeasurementFailureIndication:
		return "E-CIDMeasurementFailureIndication"
	default:
		return "Unknown"
	}
}

func buildDownlinkUEAssociatedLPPaTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseDownlinkUEAssociatedLPPaTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Downlink LPPa Transport: %v", err)}, ""
	}

	return lppaTransportValue(m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.UnknownIEs()),
		fmt.Sprintf("Downlink UE Associated LPPa Transport (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildUplinkUEAssociatedLPPaTransport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUplinkUEAssociatedLPPaTransport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Uplink LPPa Transport: %v", err)}, ""
	}

	return lppaTransportValue(m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.UnknownIEs()),
		fmt.Sprintf("Uplink UE Associated LPPa Transport (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func lppaTransportValue(mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID, routing s1ap.RoutingID, pdu s1ap.LPPaPDU, unknown []s1ap.RawIE) S1APMessageValue {
	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(mmeID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(enbID)),
		ie(idRoutingID, s1ap.CriticalityReject, uint8(routing)),
		ie(idLPPaPDU, s1ap.CriticalityReject, decodeLPPaPDU(pdu)),
	}

	ies = appendUnknownIEs(ies, unknown)

	return S1APMessageValue{IEs: ies}
}
