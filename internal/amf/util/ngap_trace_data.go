package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/aper"
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

	plmnID := models.PlmnId{}
	plmnID.Mcc = subStringSlice[0][:3]
	plmnID.Mnc = subStringSlice[0][3:]
	var traceID []byte
	traceIDTmp, err := hex.DecodeString(subStringSlice[1])
	if err != nil {
		return traceActivation, fmt.Errorf("could not decode traceID: %+v", err)
	}
	traceID = traceIDTmp

	tmp, err := PlmnIdToNgap(plmnID)
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
	case models.TraceDepth_MINIMUM:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMinimum
	case models.TraceDepth_MEDIUM:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMedium
	case models.TraceDepth_MAXIMUM:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMaximum
	case models.TraceDepth_MINIMUM_WO_VENDOR_EXTENSION:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMinimumWithoutVendorSpecificExtension
	case models.TraceDepth_MEDIUM_WO_VENDOR_EXTENSION:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMediumWithoutVendorSpecificExtension
	case models.TraceDepth_MAXIMUM_WO_VENDOR_EXTENSION:
		traceActivation.TraceDepth.Value = ngapType.TraceDepthPresentMaximumWithoutVendorSpecificExtension
	}

	return traceActivation, nil
}
