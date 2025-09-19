package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

func TraceDataToNgap(traceData models.TraceData, trsr string) (ngapType.TraceActivation, error) {
	var traceActivation ngapType.TraceActivation
	if len(trsr) != 4 {
		return traceActivation, fmt.Errorf("trace Recording Session Reference should be 2 octets")
	}
	// NG-RAN Trace ID (left most 6 octet Trace Reference + last 2 octet Trace Recoding Session Reference)
	subStringSlice := strings.Split(traceData.TraceRef, "-")

	if len(subStringSlice) != 2 {
		return traceActivation, fmt.Errorf("trace reference format is not correct")
	}

	plmnID := models.PlmnID{}
	plmnID.Mcc = subStringSlice[0][:3]
	plmnID.Mnc = subStringSlice[0][3:]
	var traceID []byte
	traceIDTmp, err := hex.DecodeString(subStringSlice[1])
	if err != nil {
		return traceActivation, fmt.Errorf("could not decode traceID: %+v", err)
	}
	traceID = traceIDTmp

	tmp, err := PlmnIDToNgap(plmnID)
	if err != nil {
		return traceActivation, fmt.Errorf("convert plmnID to NGAP failed: %+v", err)
	}
	traceReference := append(tmp.Value, traceID...)
	var trsrNgap []byte
	trsrNgapTmp, err := hex.DecodeString(trsr)
	if err != nil {
		return traceActivation, fmt.Errorf("decode trsr failed: %+v", err)
	}
	trsrNgap = trsrNgapTmp

	nGRANTraceID := append(traceReference, trsrNgap...)
	traceActivation.NGRANTraceID.Value = nGRANTraceID

	// Interfaces To Trace
	var interfacesToTrace []byte
	interfacesToTraceTmp, err := hex.DecodeString(traceData.InterfaceList)
	if err != nil {
		return traceActivation, fmt.Errorf("decode Interface failed: %+v", err)
	}
	interfacesToTrace = interfacesToTraceTmp
	traceActivation.InterfacesToTrace.Value = aper.BitString{
		Bytes:     interfacesToTrace,
		BitLength: 8,
	}

	// Trace Collection Entity IP Address
	ngapIP, err := IPAddressToNgap(traceData.CollectionEntityIpv4Addr, traceData.CollectionEntityIpv6Addr)
	if err != nil {
		return traceActivation, fmt.Errorf("could not convert IP address to NGAP: %+v", err)
	}
	traceActivation.TraceCollectionEntityIPAddress = *ngapIP

	// Trace Depth
	switch traceData.TraceDepth {
	case models.TraceDepthMinimum:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMinimum
	case models.TraceDepthMedium:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMedium
	case models.TraceDepthMaximum:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMaximum
	case models.TraceDepthMinimumWoVendorExtension:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMinimumWithoutVendorSpecificExtension
	case models.TraceDepthMediumWoVendorExtension:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMediumWithoutVendorSpecificExtension
	case models.TraceDepthMaximumWoVendorExtension:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMaximumWithoutVendorSpecificExtension
	}

	return traceActivation, nil
}
