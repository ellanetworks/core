// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

type TraceActivation struct {
	EUTRANTraceID                  string   `json:"eutran_trace_id"`
	InterfacesToTrace              []string `json:"interfaces_to_trace"`
	TraceDepth                     string   `json:"trace_depth"`
	TraceCollectionEntityIPAddress string   `json:"trace_collection_entity_ip_address"`
}

var interfacesToTraceNames = []struct {
	bit  uint8
	name string
}{
	{s1ap.InterfaceToTraceS1MME, "S1-MME"},
	{s1ap.InterfaceToTraceX2, "X2"},
	{s1ap.InterfaceToTraceUu, "Uu"},
	{s1ap.InterfaceToTraceF1C, "F1-C"},
	{s1ap.InterfaceToTraceE1, "E1"},
}

var traceDepthNames = []string{
	"minimum",
	"medium",
	"maximum",
	"minimumWithoutVendorSpecificExtension",
	"mediumWithoutVendorSpecificExtension",
	"maximumWithoutVendorSpecificExtension",
}

func traceActivation(ta s1ap.TraceActivation) TraceActivation {
	interfaces := []string{}

	for _, it := range interfacesToTraceNames {
		if ta.InterfacesToTrace&it.bit != 0 {
			interfaces = append(interfaces, it.name)
		}
	}

	depth := fmt.Sprintf("unknown (%d)", ta.TraceDepth)
	if int(ta.TraceDepth) < len(traceDepthNames) {
		depth = traceDepthNames[ta.TraceDepth]
	}

	return TraceActivation{
		EUTRANTraceID:                  hex.EncodeToString(ta.EUTRANTraceID[:]),
		InterfacesToTrace:              interfaces,
		TraceDepth:                     depth,
		TraceCollectionEntityIPAddress: transportLayerAddress(ta.TraceCollectionEntityIPAddress),
	}
}

func buildTraceStart(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseTraceStart(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Trace Start: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if m.TraceActivation != nil {
		ies = append(ies, ie(s1ap.IDTraceActivation, s1ap.CriticalityIgnore, traceActivation(*m.TraceActivation)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Trace Start (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildTraceFailureIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseTraceFailureIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Trace Failure Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if m.EUTRANTraceID != nil {
		ies = append(ies, ie(s1ap.IDEUTRANTraceID, s1ap.CriticalityIgnore, hex.EncodeToString(m.EUTRANTraceID[:])))
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Trace Failure Indication (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}
