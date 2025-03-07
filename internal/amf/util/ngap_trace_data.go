package util

import (
	"encoding/hex"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapType"
)

func TraceDataToNgap(traceData models.TraceData, trsr string) ngapType.TraceActivation {
	var traceActivation ngapType.TraceActivation

	if len(trsr) != 4 {
		logger.AmfLog.Warnln("trace Recording Session Reference should be 2 octets")
		return traceActivation
	}

	// NG-RAN Trace ID (left most 6 octet Trace Reference + last 2 octet Trace Recoding Session Reference)
	subStringSlice := strings.Split(traceData.TraceRef, "-")

	if len(subStringSlice) != 2 {
		logger.AmfLog.Warnln("traceRef format is not correct")
		return traceActivation
	}

	plmnID := models.PlmnID{}
	plmnID.Mcc = subStringSlice[0][:3]
	plmnID.Mnc = subStringSlice[0][3:]
	var traceID []byte
	if traceIDTmp, err := hex.DecodeString(subStringSlice[1]); err != nil {
		logger.AmfLog.Warnf("traceIDTmp is empty")
	} else {
		traceID = traceIDTmp
	}

	tmp := PlmnIDToNgap(plmnID)
	traceReference := append(tmp.Value, traceID...)
	var trsrNgap []byte
	if trsrNgapTmp, err := hex.DecodeString(trsr); err != nil {
		logger.AmfLog.Warnf("decode trsr failed: %+v", err)
	} else {
		trsrNgap = trsrNgapTmp
	}

	nGRANTraceID := append(traceReference, trsrNgap...)

	traceActivation.NGRANTraceID.Value = nGRANTraceID

	// Interfaces To Trace
	var interfacesToTrace []byte
	if interfacesToTraceTmp, err := hex.DecodeString(traceData.InterfaceList); err != nil {
		logger.AmfLog.Warnf("decode Interface failed: %+v", err)
	} else {
		interfacesToTrace = interfacesToTraceTmp
	}
	traceActivation.InterfacesToTrace.Value = aper.BitString{
		Bytes:     interfacesToTrace,
		BitLength: 8,
	}

	// Trace Collection Entity IP Address
	ngapIP := IPAddressToNgap(traceData.CollectionEntityIpv4Addr, traceData.CollectionEntityIpv6Addr)
	traceActivation.TraceCollectionEntityIPAddress = ngapIP

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

	return traceActivation
}
