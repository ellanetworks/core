// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

func buildUEContextModificationRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Request: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.UEAggregateMaximumBitRate != nil {
		ies = append(ies, ie(ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityIgnore, UEAggregateMaximumBitRate{
			Downlink: int64(m.UEAggregateMaximumBitRate.DL),
			Uplink:   int64(m.UEAggregateMaximumBitRate.UL),
			Unit:     "bps",
		}))
	}

	if m.NewAMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDNewAMFUENGAPID, ngap.CriticalityReject, int64(*m.NewAMFUENGAPID)))
	}

	ies = appendTyped(ies, []typedIE{
		{ngap.IDRANPagingPriority, ngap.CriticalityIgnore, m.RANPagingPriority},
		{ngap.IDSecurityKey, ngap.CriticalityReject, m.SecurityKey},
		{ngap.IDIndexToRFSP, ngap.CriticalityIgnore, m.IndexToRFSP},
		{ngap.IDUESecurityCapabilities, ngap.CriticalityReject, m.UESecurityCapabilities},
		{ngap.IDCoreNetworkAssistanceInformationForInactive, ngap.CriticalityIgnore, m.CoreNetworkAssistanceInformationForInactive},
		{ngap.IDEmergencyFallbackIndicator, ngap.CriticalityReject, m.EmergencyFallbackIndicator},
		{ngap.IDRRCInactiveTransitionReportRequest, ngap.CriticalityIgnore, m.RRCInactiveTransitionReportRequest},
		{ngap.IDNewGUAMI, ngap.CriticalityReject, m.NewGUAMI},
		{ngap.IDCNAssistedRANTuning, ngap.CriticalityIgnore, m.CNAssistedRANTuning},
		{ngap.IDSRVCCOperationPossible, ngap.CriticalityIgnore, m.SRVCCOperationPossible},
		{ngap.IDIABAuthorized, ngap.CriticalityIgnore, m.IABAuthorized},
		{ngap.IDNRV2XServicesAuthorized, ngap.CriticalityIgnore, m.NRV2XServicesAuthorized},
		{ngap.IDLTEV2XServicesAuthorized, ngap.CriticalityIgnore, m.LTEV2XServicesAuthorized},
		{ngap.IDNRUESidelinkAggregateMaximumBitrate, ngap.CriticalityIgnore, m.NRUESidelinkAggregateMaximumBitrate},
		{ngap.IDLTEUESidelinkAggregateMaximumBitrate, ngap.CriticalityIgnore, m.LTEUESidelinkAggregateMaximumBitrate},
		{ngap.IDPC5QoSParameters, ngap.CriticalityIgnore, m.PC5QoSParameters},
		{ngap.IDUERadioCapabilityID, ngap.CriticalityReject, m.UERadioCapabilityID},
		{ngap.IDRGLevelWirelineAccessCharacteristics, ngap.CriticalityIgnore, m.RGLevelWirelineAccessCharacteristics},
		{ngap.IDTimeSyncAssistanceInfo, ngap.CriticalityIgnore, m.TimeSyncAssistanceInfo},
		{ngap.IDQMCConfigInfo, ngap.CriticalityIgnore, m.QMCConfigInfo},
		{ngap.IDQMCDeactivation, ngap.CriticalityIgnore, m.QMCDeactivation},
		{ngap.IDUESliceMaximumBitRateList, ngap.CriticalityIgnore, m.UESliceMaximumBitRateList},
		{ngap.IDManagementBasedMDTPLMNModificationList, ngap.CriticalityIgnore, m.ManagementBasedMDTPLMNModificationList},
		{ngap.IDFiveGProSeAuthorized, ngap.CriticalityIgnore, m.FiveGProSeAuthorized},
		{ngap.IDFiveGProSeUEPC5AggregateMaximumBitRate, ngap.CriticalityIgnore, m.FiveGProSeUEPC5AggregateMaximumBitRate},
		{ngap.IDFiveGProSePC5QoSParameters, ngap.CriticalityIgnore, m.FiveGProSePC5QoSParameters},
		{ngap.IDNetworkControlledRepeaterAuthorized, ngap.CriticalityIgnore, m.NetworkControlledRepeaterAuthorized},
		{ngap.IDAerialUEsubscriptionInformation, ngap.CriticalityIgnore, m.AerialUEsubscriptionInformation},
		{ngap.IDNRA2XServicesAuthorized, ngap.CriticalityIgnore, m.NRA2XServicesAuthorized},
		{ngap.IDLTEA2XServicesAuthorized, ngap.CriticalityIgnore, m.LTEA2XServicesAuthorized},
		{ngap.IDNRA2XUEPC5AggregateMaximumBitRate, ngap.CriticalityIgnore, m.NRA2XUEPC5AggregateMaximumBitRate},
		{ngap.IDLTEA2XUEPC5AggregateMaximumBitRate, ngap.CriticalityIgnore, m.LTEA2XUEPC5AggregateMaximumBitRate},
		{ngap.IDA2XPC5QoSParameters, ngap.CriticalityIgnore, m.A2XPC5QoSParameters},
		{ngap.IDMobileIABAuthorized, ngap.CriticalityIgnore, m.MobileIABAuthorized},
		{ngap.IDSLPositioningRangingServiceInfo, ngap.CriticalityIgnore, m.SLPositioningRangingServiceInfo},
	})

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextModificationResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendTyped(ies, []typedIE{
		{ngap.IDRRCState, ngap.CriticalityIgnore, m.RRCState},
	})

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextModificationFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextModificationFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Modification Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
