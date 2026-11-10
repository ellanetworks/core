// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type TraceActivation struct {
	NGRANTraceID                   string   `json:"ngran_trace_id"`
	InterfacesToTrace              []string `json:"interfaces_to_trace"`
	TraceDepth                     string   `json:"trace_depth"`
	TraceCollectionEntityIPAddress string   `json:"trace_collection_entity_ip_address"`
}

var interfacesToTraceNames = []struct {
	bit  uint8
	name string
}{
	{ngap.InterfaceToTraceNGC, "NG-C"},
	{ngap.InterfaceToTraceXnC, "Xn-C"},
	{ngap.InterfaceToTraceUu, "Uu"},
	{ngap.InterfaceToTraceF1C, "F1-C"},
	{ngap.InterfaceToTraceE1, "E1"},
}

var traceDepthNames = []string{
	"minimum",
	"medium",
	"maximum",
	"minimumWithoutVendorSpecificExtension",
	"mediumWithoutVendorSpecificExtension",
	"maximumWithoutVendorSpecificExtension",
}

func traceActivation(ta ngap.TraceActivation) TraceActivation {
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
		NGRANTraceID:                   hex.EncodeToString(ta.NGRANTraceID[:]),
		InterfacesToTrace:              interfaces,
		TraceDepth:                     depth,
		TraceCollectionEntityIPAddress: tceAddressText(ta.TraceCollectionEntityIPAddress),
	}
}

func tceAddressText(addr ngap.TransportLayerAddress) string {
	if text := transportLayerAddressToString(addr); text != "" {
		return text
	}

	return hex.EncodeToString(addr)
}

func buildTraceStart(value []byte) NGAPMessageValue {
	m, err := ngap.ParseTraceStart(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Trace Start: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.TraceActivation != nil {
		ies = append(ies, ie(ngap.IDTraceActivation, ngap.CriticalityIgnore, traceActivation(*m.TraceActivation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildTraceFailureIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParseTraceFailureIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Trace Failure Indication: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.NGRANTraceID != nil {
		ies = append(ies, ie(ngap.IDNGRANTraceID, ngap.CriticalityIgnore, hex.EncodeToString(m.NGRANTraceID[:])))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
