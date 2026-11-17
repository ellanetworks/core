// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/asn1spec"
)

type enumShape struct {
	Root       []string
	Extensible bool
	Extension  []string
}

type enumSpecCase struct {
	asn1 string
	got  enumShape
	typ  reflect.Type
}

func enumSpec[T per.Integer](asn1 string, e *per.Enum[T]) enumSpecCase {
	return enumSpecCase{
		asn1: asn1,
		got:  enumShape{Root: e.Root, Extensible: e.Extensible || len(e.Extension) > 0, Extension: e.Extension},
		typ:  reflect.TypeFor[T](),
	}
}

func asn1Enum(t *testing.T, ref string) enumShape {
	t.Helper()

	s := vendoredSpec(t)

	typeName, component, _ := strings.Cut(ref, ".")

	ta := s.Type(typeName)
	if ta == nil {
		t.Fatalf("ASN.1 type %s not found", typeName)
	}

	ty := ta.Type

	if component != "" {
		c := ty.Component(component)
		if c == nil {
			t.Fatalf("%s has no component %s", typeName, component)
		}

		ty = c.Type
	}

	ty, err := s.Underlying(ty)
	if err != nil {
		t.Fatal(err)
	}

	if ty.Kind != asn1spec.Enumerated {
		t.Fatalf("%s is %s, not ENUMERATED", ref, ty.Kind)
	}

	var out enumShape

	for _, it := range ty.Enum {
		if it.HasNumber {
			t.Fatalf("%s: %s has an explicit number", ref, it.Name)
		}

		if it.Extension {
			out.Extension = append(out.Extension, it.Name)
		} else {
			out.Root = append(out.Root, it.Name)
		}
	}

	out.Extensible = ty.Extensible

	return out
}

func enumSpecCases() []enumSpecCase {
	return []enumSpecCase{
		enumSpec("CauseRadioNetwork", &enumCauseRadioNetwork),
		enumSpec("CauseTransport", &enumCauseTransport),
		enumSpec("CauseNas", &enumCauseNas),
		enumSpec("CauseProtocol", &enumCauseProtocol),
		enumSpec("CauseMisc", &enumCauseMisc),
		enumSpec("TriggeringMessage", &enumTriggeringMessage),
		enumSpec("TypeOfError", &enumTypeOfError),
		enumSpec("UEPresence", &enumUEPresence),
		enumSpec("DelayCritical", &enumDelayCritical),
		enumSpec("ReflectiveQosAttribute", &enumReflectiveQosAttribute),
		enumSpec("AdditionalQosFlowInformation", &enumAdditionalQosFlowInformation),
		enumSpec("Pre-emptionCapability", &enumPreemptionCapability),
		enumSpec("Pre-emptionVulnerability", &enumPreemptionVulnerability),
		enumSpec("NotificationControl", &enumNotificationControl),
		enumSpec("AssociatedQosFlowItem.qosFlowMappingIndication", &enumQosFlowMappingIndication),
		enumSpec("DataForwardingAccepted", &enumDataForwardingAccepted),
		enumSpec("IntegrityProtectionIndication", &enumIntegrityProtectionIndication),
		enumSpec("ConfidentialityProtectionIndication", &enumConfidentialityProtectionIndication),
		enumSpec("MaximumIntegrityProtectedDataRate", &enumMaximumIntegrityProtectedDataRate),
		enumSpec("IntegrityProtectionResult", &enumIntegrityProtectionResult),
		enumSpec("ConfidentialityProtectionResult", &enumConfidentialityProtectionResult),
		enumSpec("DirectForwardingPathAvailability", &enumDirectForwardingPathAvailability),
		enumSpec("DL-NGU-TNLInformationReused", &enumDLNGUTNLInformationReused),
		enumSpec("NotificationCause", &enumNotificationCause),
		enumSpec("GUAMIType", &enumGUAMIType),
		enumSpec("TimerApproachForGUAMIRemoval", &enumTimerApproachForGUAMIRemoval),
		enumSpec("ConcurrentWarningMessageInd", &enumConcurrentWarningMessageInd),
		enumSpec("CancelAllWarningMessages", &enumCancelAllWarningMessages),
		enumSpec("EventType", &enumEventType),
		enumSpec("ReportArea", &enumReportArea),
		enumSpec("RRCEstablishmentCause", &enumRRCEstablishmentCause),
		enumSpec("UEContextRequest", &enumUEContextRequest),
		enumSpec("GW-Context-Release-Indication", &enumGWContextReleaseIndication),
		enumSpec("TNLAssociationUsage", &enumTNLAssociationUsage),
		enumSpec("PagingPriority", &enumPagingPriority),
		enumSpec("PagingOrigin", &enumPagingOrigin),
		enumSpec("OnboardingSupport", &enumOnboardingSupport),
		enumSpec("NotifySourceNGRANNode", &enumNotifySourceNGRANNode),
		enumSpec("OverloadAction", &enumOverloadAction),
		enumSpec("TimeToWait", &enumTimeToWait),
		enumSpec("TraceDepth", &enumTraceDepth),
		enumSpec("UERetentionInformation", &enumUERetentionInformation),
		enumSpec("PagingDRX", &enumPagingDRX),
		enumSpec("HandoverType", &enumHandoverType),
		enumSpec("NewSecurityContextInd", &enumNewSecurityContextInd),
		enumSpec("PDUSessionType", &enumPDUSessionType),
		enumSpec("DLForwarding", &enumDLForwarding),
		enumSpec("CellSize", &enumCellSize),
		enumSpec("DataForwardingNotPossible", &enumDataForwardingNotPossible),
		enumSpec("UserPlaneFailureType", &enumUserPlaneFailureType),
		enumSpec("ResetAll", &enumResetAll),
		enumSpec("LineType", &enumLineType),
		enumSpec("ConfiguredTACIndication", &enumConfiguredTACIndication),
		enumSpec("RAT-Information", &enumRATInformation),
		enumSpec("LocationReportingAdditionalInfo", &enumLocationReportingAdditionalInfo),
		enumSpec("AerialUEReportingPeriodicity", &enumAerialUEReportingPeriodicity),
		enumSpec("MBS-SupportIndicator", &enumMBSSupportIndicator),
		enumSpec("PDUSetbasedHandlingIndicator", &enumPDUSetbasedHandlingIndicator),
		enumSpec("CNTypeRestrictionsForServing", &enumCNTypeRestrictionsForServing),
		enumSpec("QosMonitoringRequest", &enumQosMonitoringRequest),
		enumSpec("DLPDUSetInformationMarkingSupportIndication", &enumDLPDUSetInformationMarkingSupportIndication),
		enumSpec("Indication-of-Bitrate-Adaptation", &enumIndicationofBitrateAdaptation),
		enumSpec("RedundantQosFlowIndicator", &enumRedundantQosFlowIndicator),
		enumSpec("UserPlaneFailureIndicationReport", &enumUserPlaneFailureIndicationReport),
		enumSpec("ULForwarding", &enumULForwarding),
		enumSpec("DAPSRequestInfo.dAPSIndicator", &enumDAPSRequestInfoDAPSIndicator),
		enumSpec("RSN", &enumRSN),
		enumSpec("CNTypeRestrictionsForEquivalentItem.cn-Type", &enumCNTypeRestrictionsForEquivalentItemCnType),
		enumSpec("SourceOfUEActivityBehaviourInformation", &enumSourceOfUEActivityBehaviourInformation),
		enumSpec("PDUsetQoSInformation.pduSetIntegratedHandlingInformation", &enumPDUsetQoSInformationPduSetIntegratedHandlingInformation),
		enumSpec("MonitoringRequest", &enumMonitoringRequest),
		enumSpec("ECNMarkingAtRANRequest", &enumECNMarkingAtRANRequest),
		enumSpec("ECNMarkingAtUPFRequest", &enumECNMarkingAtUPFRequest),
		enumSpec("CongestionInformationRequest", &enumCongestionInformationRequest),
		enumSpec("PDUSessionUsageReport.rATType", &enumPDUSessionUsageReportRATType),
		enumSpec("MN-only-MDT-collection", &enumMNonlyMDTcollection),
		enumSpec("ActivationStatus", &enumActivationStatus),
		enumSpec("QoSFlowsUsageReport-Item.rATType", &enumQoSFlowsUsageReportItemRATType),
		enumSpec("MDT-Activation", &enumMDTActivation),
		enumSpec("Allowed-PNI-NPN-Item.pNI-NPN-restricted", &enumAllowedPNINPNItemPNINPNrestricted),
		enumSpec("RANfeedbacktype-reactive.capabilityForBATAdaptation", &enumRANfeedbacktypereactiveCapabilityForBATAdaptation),
		enumSpec("UEAppLayerMeasConfigInfo.qoEMeasurementStatus", &enumUEAppLayerMeasConfigInfoQoEMeasurementStatus),
		enumSpec("BluetoothMeasurementConfiguration.bt-rssi", &enumBluetoothMeasurementConfigurationBtrssi),
		enumSpec("WLANMeasurementConfiguration.wlan-rssi", &enumWLANMeasurementConfigurationWlanrssi),
		enumSpec("WLANMeasurementConfiguration.wlan-rtt", &enumWLANMeasurementConfigurationWlanrtt),
		enumSpec("LoggingInterval", &enumLoggingInterval),
		enumSpec("LoggingDuration", &enumLoggingDuration),
		enumSpec("EarlyMeasurement", &enumEarlyMeasurement),
		enumSpec("ServiceType", &enumServiceType),
		enumSpec("AvailableRANVisibleQoEMetrics.applicationLayerBufferLevelList", &enumAvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList),
		enumSpec("AvailableRANVisibleQoEMetrics.playoutDelayForMediaStartup", &enumAvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup),
		enumSpec("CommServiceType", &enumCommServiceType),
		enumSpec("QoERVQoEReportingPaths.qoEReportingPath", &enumQoERVQoEReportingPathsQoEReportingPath),
		enumSpec("QoERVQoEReportingPaths.rVQoEReportingPath", &enumQoERVQoEReportingPathsRVQoEReportingPath),
		enumSpec("M1ReportingTrigger", &enumM1ReportingTrigger),
		enumSpec("IncludeBeamMeasurementsIndication", &enumIncludeBeamMeasurementsIndication),
		enumSpec("M4period", &enumM4period),
		enumSpec("Links-to-log", &enumLinkstolog),
		enumSpec("M4ReportAmountMDT", &enumM4ReportAmountMDT),
		enumSpec("M5period", &enumM5period),
		enumSpec("M5ReportAmountMDT", &enumM5ReportAmountMDT),
		enumSpec("M6report-Interval", &enumM6reportInterval),
		enumSpec("M6ReportAmountMDT", &enumM6ReportAmountMDT),
		enumSpec("M7ReportAmountMDT", &enumM7ReportAmountMDT),
		enumSpec("BluetoothMeasConfig", &enumBluetoothMeasConfig),
		enumSpec("WLANMeasConfig", &enumWLANMeasConfig),
		enumSpec("SensorMeasConfig", &enumSensorMeasConfig),
		enumSpec("EventTrigger.outOfCoverage", &enumEventTriggerOutOfCoverage),
		enumSpec("ReportIntervalMDT", &enumReportIntervalMDT),
		enumSpec("ReportAmountMDT", &enumReportAmountMDT),
		enumSpec("ExtendedReportIntervalMDT", &enumExtendedReportIntervalMDT),
		enumSpec("BeamMeasurementsReportQuantity.rSRP", &enumBeamMeasurementsReportQuantityRSRP),
		enumSpec("BeamMeasurementsReportQuantity.rSRQ", &enumBeamMeasurementsReportQuantityRSRQ),
		enumSpec("BeamMeasurementsReportQuantity.sINR", &enumBeamMeasurementsReportQuantitySINR),
		enumSpec("ExcessPacketDelayThresholdValue", &enumExcessPacketDelayThresholdValue),
		enumSpec("SensorNameConfig.uncompensatedBarometricConfig", &enumSensorNameConfigUncompensatedBarometricConfig),
		enumSpec("SensorNameConfig.ueSpeedConfig", &enumSensorNameConfigUeSpeedConfig),
		enumSpec("SensorNameConfig.ueOrientationConfig", &enumSensorNameConfigUeOrientationConfig),
		enumSpec("TimeToTrigger", &enumTimeToTrigger),
		enumSpec("RRCInactiveTransitionReportRequest", &enumRRCInactiveTransitionReportRequest),
		enumSpec("RedirectionVoiceFallback", &enumRedirectionVoiceFallback),
		enumSpec("SRVCCOperationPossible", &enumSRVCCOperationPossible),
		enumSpec("IAB-Authorized", &enumIABAuthorized),
		enumSpec("Enhanced-CoverageRestriction", &enumEnhancedCoverageRestriction),
		enumSpec("UE-DifferentiationInfo.periodicCommunicationIndicator", &enumUEDifferentiationInfoPeriodicCommunicationIndicator),
		enumSpec("UE-DifferentiationInfo.stationaryIndication", &enumUEDifferentiationInfoStationaryIndication),
		enumSpec("UE-DifferentiationInfo.trafficProfile", &enumUEDifferentiationInfoTrafficProfile),
		enumSpec("UE-DifferentiationInfo.batteryIndication", &enumUEDifferentiationInfoBatteryIndication),
		enumSpec("CEmodeBrestricted", &enumCEmodeBrestricted),
		enumSpec("UE-UP-CIoT-Support", &enumUEUPCIoTSupport),
		enumSpec("TimeSyncAssistanceInfo.timeDistributionIndication", &enumTimeSyncAssistanceInfoTimeDistributionIndication),
		enumSpec("AerialUEsubscriptionInformation", &enumAerialUEsubscriptionInformation),
		enumSpec("NR-A2X-ServicesAuthorized.aerialUE", &enumNRA2XServicesAuthorizedAerialUE),
		enumSpec("NR-A2X-ServicesAuthorized.aerialControllerUE", &enumNRA2XServicesAuthorizedAerialControllerUE),
		enumSpec("LTE-A2X-ServicesAuthorized.aerialUE", &enumLTEA2XServicesAuthorizedAerialUE),
		enumSpec("LTE-A2X-ServicesAuthorized.aerialControllerUE", &enumLTEA2XServicesAuthorizedAerialControllerUE),
		enumSpec("MobileIAB-Authorized", &enumMobileIABAuthorized),
		enumSpec("NoPDUSessionIndication", &enumNoPDUSessionIndication),
		enumSpec("RedCapIndication", &enumRedCapIndication),
		enumSpec("ERedCapIndication", &enumERedCapIndication),
		enumSpec("NetworkControlledRepeaterAuthorized", &enumNetworkControlledRepeaterAuthorized),
		enumSpec("IABNodeIndication", &enumIABNodeIndication),
		enumSpec("CEmodeBSupport-Indicator", &enumCEmodeBSupportIndicator),
		enumSpec("LTEM-Indication", &enumLTEMIndication),
		enumSpec("EDT-Session", &enumEDTSession),
		enumSpec("AuthenticatedIndication", &enumAuthenticatedIndication),
		enumSpec("MobileIABNodeIndication", &enumMobileIABNodeIndication),
		enumSpec("AUN3DeviceAccessInfo.aUN3DeviceAccess", &enumAUN3DeviceAccessInfoAUN3DeviceAccess),
		enumSpec("AUN3DeviceAccessInfo.existingNGConnectionwithSameLocation", &enumAUN3DeviceAccessInfoExistingNGConnectionwithSameLocation),
		enumSpec("UECapabilityInfoRequest", &enumUECapabilityInfoRequest),
		enumSpec("EndIndication", &enumEndIndication),
		enumSpec("NB-IoT-DefaultPagingDRX", &enumNBIoTDefaultPagingDRX),
		enumSpec("AIoT-Support", &enumAIoTSupport),
		enumSpec("IAB-Supported", &enumIABSupported),
		enumSpec("MobileIAB-Supported", &enumMobileIABSupported),
		enumSpec("NB-IoT-PagingDRX", &enumNBIoTPagingDRX),
		enumSpec("PagingCause", &enumPagingCause),
		enumSpec("LPWUSDisableIndication", &enumLPWUSDisableIndication),
		enumSpec("RRCState", &enumRRCState),
		enumSpec("XrDeviceWith2Rx", &enumXrDeviceWith2Rx),
		enumSpec("MICOModeIndication", &enumMICOModeIndication),
		enumSpec("MicoAllPLMN", &enumMicoAllPLMN),
		enumSpec("PagingCauseIndicationForVoiceService", &enumPagingCauseIndicationForVoiceService),
		enumSpec("CN-MT-CommunicationHandling", &enumCNMTCommunicationHandling),
		enumSpec("VehicleUE", &enumVehicleUE),
		enumSpec("PedestrianUE", &enumPedestrianUE),
		enumSpec("FiveGProSeDirectDiscovery", &enumFiveGProSeDirectDiscovery),
		enumSpec("FiveGProSeDirectCommunication", &enumFiveGProSeDirectCommunication),
		enumSpec("FiveGProSeLayer2UEtoNetworkRelay", &enumFiveGProSeLayer2UEtoNetworkRelay),
		enumSpec("FiveGProSeLayer3UEtoNetworkRelay", &enumFiveGProSeLayer3UEtoNetworkRelay),
		enumSpec("FiveGProSeLayer2RemoteUE", &enumFiveGProSeLayer2RemoteUE),
		enumSpec("FiveGProSeLayer2Multipath", &enumFiveGProSeLayer2Multipath),
		enumSpec("FiveGProSeLayer2UEtoUERelay", &enumFiveGProSeLayer2UEtoUERelay),
		enumSpec("FiveGProSeLayer2UEtoUERemote", &enumFiveGProSeLayer2UEtoUERemote),
		enumSpec("FiveGProSeLayer2MHUEtoNetworkRelay", &enumFiveGProSeLayer2MHUEtoNetworkRelay),
		enumSpec("FiveGProSeLayer2MHIntermediateUEtoNetworkRelay", &enumFiveGProSeLayer2MHIntermediateUEtoNetworkRelay),
		enumSpec("FiveGProSeLayer2MHRemote", &enumFiveGProSeLayer2MHRemote),
		enumSpec("FiveGProSeLayer3MHUEtoNetworkRelay", &enumFiveGProSeLayer3MHUEtoNetworkRelay),
		enumSpec("SLPositioningRangingAuthorized", &enumSLPositioningRangingAuthorized),
		enumSpec("EmergencyFallbackRequestIndicator", &enumEmergencyFallbackRequestIndicator),
		enumSpec("EmergencyServiceTargetCN", &enumEmergencyServiceTargetCN),
		enumSpec("NB-IoT-Paging-eDRXCycle", &enumNBIoTPagingeDRXCycle),
		enumSpec("NB-IoT-Paging-TimeWindow", &enumNBIoTPagingTimeWindow),
		enumSpec("PagingProbabilityInformation", &enumPagingProbabilityInformation),
		enumSpec("EUTRA-Paging-eDRX-Cycle", &enumEUTRAPagingeDRXCycle),
		enumSpec("EUTRA-Paging-Time-Window", &enumEUTRAPagingTimeWindow),
		enumSpec("NR-Paging-eDRX-Cycle", &enumNRPagingeDRXCycle),
		enumSpec("NR-Paging-Time-Window", &enumNRPagingTimeWindow),
		enumSpec("ExpectedHOInterval", &enumExpectedHOInterval),
		enumSpec("ExpectedUEMobility", &enumExpectedUEMobility),
		enumSpec("A2X-PC5-QoS-Flow-Item.a2X-Range", &enumA2XPC5QoSFlowItemA2XRange),
		enumSpec("NextPagingAreaScope", &enumNextPagingAreaScope),
		enumSpec("MBS-AssistanceInformation", &enumMBSAssistanceInformation),
		enumSpec("Range", &enumRangeType),
		enumSpec("ClockQualityAcceptanceCriteria.traceabletoUTC", &enumClockQualityAcceptanceCriteriaTraceabletoUTC),
		enumSpec("ClockQualityAcceptanceCriteria.traceabletoGNSS", &enumClockQualityAcceptanceCriteriaTraceabletoGNSS),
		enumSpec("IntersystemUnnecessaryHO.earlyIRATHO", &enumIntersystemUnnecessaryHOEarlyIRATHO),
		enumSpec("NotificationCell-Item.notifyFlag", &enumNotificationCellItemNotifyFlag),
		enumSpec("NumberOfMeasurementReportingLevels", &enumNumberOfMeasurementReportingLevels),
		enumSpec("ReportingPeriodicity", &enumReportingPeriodicity),
	}
}

func TestEnumLiteralsMatchASN1(t *testing.T) {
	for _, c := range enumSpecCases() {
		t.Run(c.asn1, func(t *testing.T) {
			want := asn1Enum(t, c.asn1)

			if !slices.Equal(c.got.Root, want.Root) || c.got.Extensible != want.Extensible || !slices.Equal(c.got.Extension, want.Extension) {
				t.Errorf("modeled as %+v, spec says %+v", c.got, want)
			}
		})
	}
}

func TestEnumCasesAreComplete(t *testing.T) {
	cases := enumSpecCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "per.Enum", scannedDecls(t).enums, types)
}

func TestCriticalityLiteralsMatchASN1(t *testing.T) {
	want := asn1Enum(t, "Criticality")

	got := enumShape{}
	for _, c := range []Criticality{CriticalityReject, CriticalityIgnore, CriticalityNotify} {
		got.Root = append(got.Root, c.String())
	}

	if !slices.Equal(got.Root, want.Root) || want.Extensible || len(want.Root) != criticalityRootCount {
		t.Errorf("Criticality = %+v, spec says %+v", got, want)
	}

	for i, c := range []Criticality{CriticalityReject, CriticalityIgnore, CriticalityNotify} {
		if int(c) != i {
			t.Errorf("%s has index %d, spec says %d", c, int(c), i)
		}
	}
}
