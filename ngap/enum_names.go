// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

// ASN.1 enumeration item names from TS 38.413 §9.4.5 and §9.4.6. A Name backed
// by a map returns the empty string for a value the module does not name; one
// generated from a per.Enum returns "unknown(n)".

var enumTriggeringMessage = per.Enum[TriggeringMessage]{
	Root: []string{"initiating-message", "successful-outcome", "unsuccessful-outcome"},
}

var enumTypeOfError = per.Enum[TypeOfError]{
	Root:       []string{"not-understood", "missing"},
	Extensible: true,
}

var enumUEPresence = per.Enum[UEPresence]{
	Root:       []string{"in", "out", "unknown"},
	Extensible: true,
}

var enumDelayCritical = per.Enum[DelayCritical]{
	Root:       []string{"delay-critical", "non-delay-critical"},
	Extensible: true,
}

var enumReflectiveQosAttribute = per.Enum[ReflectiveQosAttribute]{
	Root:       []string{"subject-to"},
	Extensible: true,
}

var enumAdditionalQosFlowInformation = per.Enum[AdditionalQosFlowInformation]{
	Root:       []string{"more-likely"},
	Extensible: true,
}

var enumPreemptionCapability = per.Enum[PreemptionCapability]{
	Root:       []string{"shall-not-trigger-pre-emption", "may-trigger-pre-emption"},
	Extensible: true,
}

var enumPreemptionVulnerability = per.Enum[PreemptionVulnerability]{
	Root:       []string{"not-pre-emptable", "pre-emptable"},
	Extensible: true,
}

var enumNotificationControl = per.Enum[NotificationControl]{
	Root:       []string{"notification-requested"},
	Extensible: true,
}

var enumQosFlowMappingIndication = per.Enum[QosFlowMappingIndication]{
	Root:       []string{"ul", "dl"},
	Extensible: true,
}

var enumDataForwardingAccepted = per.Enum[DataForwardingAccepted]{
	Root:       []string{"data-forwarding-accepted"},
	Extensible: true,
}

var enumIntegrityProtectionIndication = per.Enum[IntegrityProtectionIndication]{
	Root:       []string{"required", "preferred", "not-needed"},
	Extensible: true,
}

var enumConfidentialityProtectionIndication = per.Enum[ConfidentialityProtectionIndication]{
	Root:       []string{"required", "preferred", "not-needed"},
	Extensible: true,
}

var enumMaximumIntegrityProtectedDataRate = per.Enum[MaximumIntegrityProtectedDataRate]{
	Root:       []string{"bitrate64kbs", "maximum-UE-rate"},
	Extensible: true,
}

var enumIntegrityProtectionResult = per.Enum[IntegrityProtectionResult]{
	Root:       []string{"performed", "not-performed"},
	Extensible: true,
}

var enumConfidentialityProtectionResult = per.Enum[ConfidentialityProtectionResult]{
	Root:       []string{"performed", "not-performed"},
	Extensible: true,
}

var enumDirectForwardingPathAvailability = per.Enum[DirectForwardingPathAvailability]{
	Root:       []string{"direct-path-available"},
	Extensible: true,
}

var enumDLNGUTNLInformationReused = per.Enum[DLNGUTNLInformationReused]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumNotificationCause = per.Enum[NotificationCause]{
	Root:      []string{"fulfilled", "not-fulfilled"},
	Extension: []string{"not-fulfilled-DL", "not-fulfilled-UL"},
}

var enumGUAMIType = per.Enum[GUAMIType]{
	Root:       []string{"native", "mapped"},
	Extensible: true,
}

var enumTimerApproachForGUAMIRemoval = per.Enum[TimerApproachForGUAMIRemoval]{
	Root:       []string{"apply-timer"},
	Extensible: true,
}

var enumConcurrentWarningMessageInd = per.Enum[ConcurrentWarningMessageInd]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumCancelAllWarningMessages = per.Enum[CancelAllWarningMessages]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumEventType = per.Enum[EventType]{
	Root:      []string{"direct", "change-of-serve-cell", "ue-presence-in-area-of-interest", "stop-change-of-serve-cell", "stop-ue-presence-in-area-of-interest", "cancel-location-reporting-for-the-ue"},
	Extension: []string{"change-of-serving-cell-and-UE-presence-in-the-Area-of-Interest", "report-the-Aerial-UE-flight-information", "cancel-the-Aerial-UE-flight-information-reporting"},
}

var enumReportArea = per.Enum[ReportArea]{
	Root:       []string{"cell"},
	Extensible: true,
}

var enumRRCEstablishmentCause = per.Enum[RRCEstablishmentCause]{
	Root:      []string{"emergency", "highPriorityAccess", "mt-Access", "mo-Signalling", "mo-Data", "mo-VoiceCall", "mo-VideoCall", "mo-SMS", "mps-PriorityAccess", "mcs-PriorityAccess"},
	Extension: []string{"notAvailable", "mo-ExceptionData"},
}

var enumUEContextRequest = per.Enum[UEContextRequest]{
	Root:       []string{"requested"},
	Extensible: true,
}

var enumGWContextReleaseIndication = per.Enum[GWContextReleaseIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumTNLAssociationUsage = per.Enum[TNLAssociationUsage]{
	Root:       []string{"ue", "non-ue", "both"},
	Extensible: true,
}

var enumPagingPriority = per.Enum[PagingPriority]{
	Root:       []string{"priolevel1", "priolevel2", "priolevel3", "priolevel4", "priolevel5", "priolevel6", "priolevel7", "priolevel8"},
	Extensible: true,
}

var enumPagingOrigin = per.Enum[PagingOrigin]{
	Root:       []string{"non-3gpp"},
	Extensible: true,
}

var enumOnboardingSupport = per.Enum[OnboardingSupport]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumNotifySourceNGRANNode = per.Enum[NotifySourceNGRANNode]{
	Root:       []string{"notifySource"},
	Extensible: true,
}

var enumOverloadAction = per.Enum[OverloadAction]{
	Root:       []string{"reject-non-emergency-mo-dt", "reject-rrc-cr-signalling", "permit-emergency-sessions-and-mobile-terminated-services-only", "permit-high-priority-sessions-and-mobile-terminated-services-only"},
	Extensible: true,
}

var enumTimeToWait = per.Enum[TimeToWait]{
	Root:       []string{"v1s", "v2s", "v5s", "v10s", "v20s", "v60s"},
	Extensible: true,
}

var enumTraceDepth = per.Enum[TraceDepth]{
	Root:      []string{"minimum", "medium", "maximum", "minimumWithoutVendorSpecificExtension", "mediumWithoutVendorSpecificExtension", "maximumWithoutVendorSpecificExtension"},
	Extension: []string{"minimumOnlyVendorSpecificTraceRecord", "mediumOnlyVendorSpecificTraceRecord", "maximumOnlyVendorSpecificTraceRecord"},
}

var enumUERetentionInformation = per.Enum[UERetentionInformation]{
	Root:       []string{"ues-retained"},
	Extensible: true,
}

var enumPagingDRX = per.Enum[PagingDRX]{
	Root:       []string{"v32", "v64", "v128", "v256"},
	Extensible: true,
}

var enumHandoverType = per.Enum[HandoverType]{
	Root:      []string{"intra5gs", "fivegs-to-eps", "eps-to-5gs"},
	Extension: []string{"fivegs-to-utran"},
}

var enumNewSecurityContextInd = per.Enum[NewSecurityContextInd]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumPDUSessionType = per.Enum[PDUSessionType]{
	Root:       []string{"ipv4", "ipv6", "ipv4v6", "ethernet", "unstructured"},
	Extensible: true,
}

var enumDLForwarding = per.Enum[DLForwarding]{
	Root:       []string{"dl-forwarding-proposed"},
	Extensible: true,
}

var enumCellSize = per.Enum[CellSize]{
	Root:       []string{"verysmall", "small", "medium", "large"},
	Extensible: true,
}

var enumDataForwardingNotPossible = per.Enum[DataForwardingNotPossible]{
	Root:       []string{"data-forwarding-not-possible"},
	Extensible: true,
}

var enumUserPlaneFailureType = per.Enum[UserPlaneFailureType]{
	Root:       []string{"gtp-u-error-indication-received", "up-path-failure"},
	Extensible: true,
}

var enumResetAll = per.Enum[ResetAll]{
	Root:       []string{"reset-all"},
	Extensible: true,
}

var enumLineType = per.Enum[LineType]{
	Root:       []string{"dsl", "pon"},
	Extensible: true,
}

var enumConfiguredTACIndication = per.Enum[ConfiguredTACIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumRATInformation = per.Enum[RATInformation]{
	Root:      []string{"unlicensed", "nb-IoT"},
	Extension: []string{"nR-LEO", "nR-MEO", "nR-GEO", "nR-OTHERSAT"},
}

var enumLocationReportingAdditionalInfo = per.Enum[LocationReportingAdditionalInfo]{
	Root:       []string{"includePSCell"},
	Extensible: true,
}

var enumAerialUEReportingPeriodicity = per.Enum[AerialUEReportingPeriodicity]{
	Root:       []string{"ms120", "ms240", "ms480", "ms640", "ms1024", "ms2048", "ms5120", "ms10240", "ms20480", "ms40960", "min1", "min6", "min12", "min30"},
	Extensible: true,
}

var enumMBSSupportIndicator = per.Enum[MBSSupportIndicator]{
	Root:      []string{"multicast-supported"},
	Extension: []string{"multicast-supported-with-reception-in-RRC-inactive"},
}

var enumPDUSetbasedHandlingIndicator = per.Enum[PDUSetbasedHandlingIndicator]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumCNTypeRestrictionsForServing = per.Enum[CNTypeRestrictionsForServing]{
	Root:       []string{"epc-forbidden"},
	Extensible: true,
}

var enumQosMonitoringRequest = per.Enum[QosMonitoringRequest]{
	Root:      []string{"ul", "dl", "both"},
	Extension: []string{"stop"},
}

var enumDLPDUSetInformationMarkingSupportIndication = per.Enum[DLPDUSetInformationMarkingSupportIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumIndicationofBitrateAdaptation = per.Enum[IndicationofBitrateAdaptation]{
	Root:       []string{"uplink"},
	Extensible: true,
}

var enumRedundantQosFlowIndicator = per.Enum[RedundantQosFlowIndicator]{
	Root: []string{"true", "false"},
}

var enumUserPlaneFailureIndicationReport = per.Enum[UserPlaneFailureIndicationReport]{
	Root:       []string{"qos-flows-and-tunnel-to-be-released", "qoS-flows-to-be-moved", "new-transport-address-allocated"},
	Extensible: true,
}

var enumULForwarding = per.Enum[ULForwarding]{
	Root:       []string{"ul-forwarding-proposed"},
	Extensible: true,
}

var enumDAPSRequestInfoDAPSIndicator = per.Enum[DAPSRequestInfoDAPSIndicator]{
	Root:       []string{"daps-ho-required"},
	Extensible: true,
}

var enumRSN = per.Enum[RSN]{
	Root:       []string{"v1", "v2"},
	Extensible: true,
}

var enumCNTypeRestrictionsForEquivalentItemCnType = per.Enum[CNTypeRestrictionsForEquivalentItemCnType]{
	Root:       []string{"epc-forbidden", "fiveGC-forbidden"},
	Extensible: true,
}

var enumSourceOfUEActivityBehaviourInformation = per.Enum[SourceOfUEActivityBehaviourInformation]{
	Root:       []string{"subscription-information", "statistics"},
	Extensible: true,
}

var enumPDUsetQoSInformationPduSetIntegratedHandlingInformation = per.Enum[PDUsetQoSInformationPduSetIntegratedHandlingInformation]{
	Root:       []string{"true", "false"},
	Extensible: true,
}

var enumMonitoringRequest = per.Enum[MonitoringRequest]{
	Root:       []string{"ul", "dl", "both", "stop"},
	Extensible: true,
}

var enumECNMarkingAtRANRequest = per.Enum[ECNMarkingAtRANRequest]{
	Root:       []string{"ul", "dl", "both", "stop"},
	Extensible: true,
}

var enumECNMarkingAtUPFRequest = per.Enum[ECNMarkingAtUPFRequest]{
	Root:       []string{"ul", "dl", "both", "stop"},
	Extensible: true,
}

var enumCongestionInformationRequest = per.Enum[CongestionInformationRequest]{
	Root:       []string{"ul", "dl", "both", "stop"},
	Extensible: true,
}

var enumPDUSessionUsageReportRATType = per.Enum[PDUSessionUsageReportRATType]{
	Root:      []string{"nr", "eutra"},
	Extension: []string{"nr-unlicensed", "e-utra-unlicensed"},
}

var enumMNonlyMDTcollection = per.Enum[MNonlyMDTcollection]{
	Root:       []string{"mn-only"},
	Extensible: true,
}

var enumActivationStatus = per.Enum[ActivationStatus]{
	Root:       []string{"active", "not-active"},
	Extensible: true,
}

var enumQoSFlowsUsageReportItemRATType = per.Enum[QoSFlowsUsageReportItemRATType]{
	Root:      []string{"nr", "eutra"},
	Extension: []string{"nr-unlicensed", "e-utra-unlicensed"},
}

var enumMDTActivation = per.Enum[MDTActivation]{
	Root:       []string{"immediate-MDT-only", "logged-MDT-only", "immediate-MDT-and-Trace"},
	Extensible: true,
}

var enumAllowedPNINPNItemPNINPNrestricted = per.Enum[AllowedPNINPNItemPNINPNrestricted]{
	Root:       []string{"restricted", "not-restricted"},
	Extensible: true,
}

var enumRANfeedbacktypereactiveCapabilityForBATAdaptation = per.Enum[RANfeedbacktypereactiveCapabilityForBATAdaptation]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumUEAppLayerMeasConfigInfoQoEMeasurementStatus = per.Enum[UEAppLayerMeasConfigInfoQoEMeasurementStatus]{
	Root:       []string{"ongoing"},
	Extensible: true,
}

var enumBluetoothMeasurementConfigurationBtrssi = per.Enum[BluetoothMeasurementConfigurationBtrssi]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumWLANMeasurementConfigurationWlanrssi = per.Enum[WLANMeasurementConfigurationWlanrssi]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumWLANMeasurementConfigurationWlanrtt = per.Enum[WLANMeasurementConfigurationWlanrtt]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumLoggingInterval = per.Enum[LoggingInterval]{
	Root:       []string{"ms320", "ms640", "ms1280", "ms2560", "ms5120", "ms10240", "ms20480", "ms30720", "ms40960", "ms61440", "infinity"},
	Extensible: true,
}

var enumLoggingDuration = per.Enum[LoggingDuration]{
	Root:       []string{"m10", "m20", "m40", "m60", "m90", "m120"},
	Extensible: true,
}

var enumEarlyMeasurement = per.Enum[EarlyMeasurement]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumServiceType = per.Enum[ServiceType]{
	Root:       []string{"streaming", "mTSI", "vR"},
	Extensible: true,
}

var enumAvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList = per.Enum[AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumAvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup = per.Enum[AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumCommServiceType = per.Enum[CommServiceType]{
	Root:      []string{"mbs-broadcast", "mbs-multicast"},
	Extension: []string{"unicast"},
}

var enumQoERVQoEReportingPathsQoEReportingPath = per.Enum[QoERVQoEReportingPathsQoEReportingPath]{
	Root:       []string{"srb4", "srb5"},
	Extensible: true,
}

var enumQoERVQoEReportingPathsRVQoEReportingPath = per.Enum[QoERVQoEReportingPathsRVQoEReportingPath]{
	Root:       []string{"srb4", "srb5"},
	Extensible: true,
}

var enumM1ReportingTrigger = per.Enum[M1ReportingTrigger]{
	Root:       []string{"periodic", "a2eventtriggered", "a2eventtriggered-periodic"},
	Extensible: true,
}

var enumIncludeBeamMeasurementsIndication = per.Enum[IncludeBeamMeasurementsIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumM4period = per.Enum[M4period]{
	Root:       []string{"ms1024", "ms2048", "ms5120", "ms10240", "min1"},
	Extensible: true,
}

var enumLinkstolog = per.Enum[Linkstolog]{
	Root:       []string{"uplink", "downlink", "both-uplink-and-downlink"},
	Extensible: true,
}

var enumM4ReportAmountMDT = per.Enum[M4ReportAmountMDT]{
	Root:       []string{"r1", "r2", "r4", "r8", "r16", "r32", "r64", "infinity"},
	Extensible: true,
}

var enumM5period = per.Enum[M5period]{
	Root:       []string{"ms1024", "ms2048", "ms5120", "ms10240", "min1"},
	Extensible: true,
}

var enumM5ReportAmountMDT = per.Enum[M5ReportAmountMDT]{
	Root:       []string{"r1", "r2", "r4", "r8", "r16", "r32", "r64", "infinity"},
	Extensible: true,
}

var enumM6reportInterval = per.Enum[M6reportInterval]{
	Root:       []string{"ms120", "ms240", "ms480", "ms640", "ms1024", "ms2048", "ms5120", "ms10240", "ms20480", "ms40960", "min1", "min6", "min12", "min30"},
	Extensible: true,
}

var enumM6ReportAmountMDT = per.Enum[M6ReportAmountMDT]{
	Root:       []string{"r1", "r2", "r4", "r8", "r16", "r32", "r64", "infinity"},
	Extensible: true,
}

var enumM7ReportAmountMDT = per.Enum[M7ReportAmountMDT]{
	Root:       []string{"r1", "r2", "r4", "r8", "r16", "r32", "r64", "infinity"},
	Extensible: true,
}

var enumBluetoothMeasConfig = per.Enum[BluetoothMeasConfig]{
	Root:       []string{"setup"},
	Extensible: true,
}

var enumWLANMeasConfig = per.Enum[WLANMeasConfig]{
	Root:       []string{"setup"},
	Extensible: true,
}

var enumSensorMeasConfig = per.Enum[SensorMeasConfig]{
	Root:       []string{"setup"},
	Extensible: true,
}

var enumEventTriggerOutOfCoverage = per.Enum[EventTriggerOutOfCoverage]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumReportIntervalMDT = per.Enum[ReportIntervalMDT]{
	Root: []string{"ms120", "ms240", "ms480", "ms640", "ms1024", "ms2048", "ms5120", "ms10240", "min1", "min6", "min12", "min30", "min60"},
}

var enumReportAmountMDT = per.Enum[ReportAmountMDT]{
	Root: []string{"r1", "r2", "r4", "r8", "r16", "r32", "r64", "rinfinity"},
}

var enumExtendedReportIntervalMDT = per.Enum[ExtendedReportIntervalMDT]{
	Root:       []string{"ms20480", "ms40960"},
	Extensible: true,
}

var enumBeamMeasurementsReportQuantityRSRP = per.Enum[BeamMeasurementsReportQuantityRSRP]{
	Root:      []string{"true"},
	Extension: []string{"false"},
}

var enumBeamMeasurementsReportQuantityRSRQ = per.Enum[BeamMeasurementsReportQuantityRSRQ]{
	Root:      []string{"true"},
	Extension: []string{"false"},
}

var enumBeamMeasurementsReportQuantitySINR = per.Enum[BeamMeasurementsReportQuantitySINR]{
	Root:      []string{"true"},
	Extension: []string{"false"},
}

var enumExcessPacketDelayThresholdValue = per.Enum[ExcessPacketDelayThresholdValue]{
	Root:       []string{"ms0dot25", "ms0dot5", "ms1", "ms2", "ms4", "ms5", "ms10", "ms20", "ms30", "ms40", "ms50", "ms60", "ms70", "ms80", "ms90", "ms100", "ms150", "ms300", "ms500"},
	Extensible: true,
}

var enumSensorNameConfigUncompensatedBarometricConfig = per.Enum[SensorNameConfigUncompensatedBarometricConfig]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumSensorNameConfigUeSpeedConfig = per.Enum[SensorNameConfigUeSpeedConfig]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumSensorNameConfigUeOrientationConfig = per.Enum[SensorNameConfigUeOrientationConfig]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumTimeToTrigger = per.Enum[TimeToTrigger]{
	Root: []string{"ms0", "ms40", "ms64", "ms80", "ms100", "ms128", "ms160", "ms256", "ms320", "ms480", "ms512", "ms640", "ms1024", "ms1280", "ms2560", "ms5120"},
}

var enumRRCInactiveTransitionReportRequest = per.Enum[RRCInactiveTransitionReportRequest]{
	Root:       []string{"subsequent-state-transition-report", "single-rrc-connected-state-report", "cancel-report"},
	Extensible: true,
}

var enumRedirectionVoiceFallback = per.Enum[RedirectionVoiceFallback]{
	Root:       []string{"possible", "not-possible"},
	Extensible: true,
}

var enumSRVCCOperationPossible = per.Enum[SRVCCOperationPossible]{
	Root:       []string{"possible", "notPossible"},
	Extensible: true,
}

var enumIABAuthorized = per.Enum[IABAuthorized]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumEnhancedCoverageRestriction = per.Enum[EnhancedCoverageRestriction]{
	Root:       []string{"restricted"},
	Extensible: true,
}

var enumUEDifferentiationInfoPeriodicCommunicationIndicator = per.Enum[UEDifferentiationInfoPeriodicCommunicationIndicator]{
	Root:       []string{"periodically", "ondemand"},
	Extensible: true,
}

var enumUEDifferentiationInfoStationaryIndication = per.Enum[UEDifferentiationInfoStationaryIndication]{
	Root:       []string{"stationary", "mobile"},
	Extensible: true,
}

var enumUEDifferentiationInfoTrafficProfile = per.Enum[UEDifferentiationInfoTrafficProfile]{
	Root:       []string{"single-packet", "dual-packets", "multiple-packets"},
	Extensible: true,
}

var enumUEDifferentiationInfoBatteryIndication = per.Enum[UEDifferentiationInfoBatteryIndication]{
	Root:       []string{"battery-powered", "battery-powered-not-rechargeable-or-replaceable", "not-battery-powered"},
	Extensible: true,
}

var enumCEmodeBrestricted = per.Enum[CEmodeBrestricted]{
	Root:       []string{"restricted", "not-restricted"},
	Extensible: true,
}

var enumUEUPCIoTSupport = per.Enum[UEUPCIoTSupport]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumTimeSyncAssistanceInfoTimeDistributionIndication = per.Enum[TimeSyncAssistanceInfoTimeDistributionIndication]{
	Root:       []string{"enabled", "disabled"},
	Extensible: true,
}

var enumAerialUEsubscriptionInformation = per.Enum[AerialUEsubscriptionInformation]{
	Root:       []string{"allowed", "not-allowed"},
	Extensible: true,
}

var enumNRA2XServicesAuthorizedAerialUE = per.Enum[NRA2XServicesAuthorizedAerialUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumNRA2XServicesAuthorizedAerialControllerUE = per.Enum[NRA2XServicesAuthorizedAerialControllerUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumLTEA2XServicesAuthorizedAerialUE = per.Enum[LTEA2XServicesAuthorizedAerialUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumLTEA2XServicesAuthorizedAerialControllerUE = per.Enum[LTEA2XServicesAuthorizedAerialControllerUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumMobileIABAuthorized = per.Enum[MobileIABAuthorized]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumNoPDUSessionIndication = per.Enum[NoPDUSessionIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumRedCapIndication = per.Enum[RedCapIndication]{
	Root:       []string{"redcap"},
	Extensible: true,
}

var enumERedCapIndication = per.Enum[ERedCapIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumNetworkControlledRepeaterAuthorized = per.Enum[NetworkControlledRepeaterAuthorized]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumIABNodeIndication = per.Enum[IABNodeIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumCEmodeBSupportIndicator = per.Enum[CEmodeBSupportIndicator]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumLTEMIndication = per.Enum[LTEMIndication]{
	Root:       []string{"lte-m"},
	Extensible: true,
}

var enumEDTSession = per.Enum[EDTSession]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumAuthenticatedIndication = per.Enum[AuthenticatedIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumMobileIABNodeIndication = per.Enum[MobileIABNodeIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumAUN3DeviceAccessInfoAUN3DeviceAccess = per.Enum[AUN3DeviceAccessInfoAUN3DeviceAccess]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumAUN3DeviceAccessInfoExistingNGConnectionwithSameLocation = per.Enum[AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation]{
	Root:       []string{"true", "false"},
	Extensible: true,
}

var enumUECapabilityInfoRequest = per.Enum[UECapabilityInfoRequest]{
	Root:       []string{"requested"},
	Extensible: true,
}

var enumEndIndication = per.Enum[EndIndication]{
	Root:       []string{"no-further-data", "further-data-exists"},
	Extensible: true,
}

var enumNBIoTDefaultPagingDRX = per.Enum[NBIoTDefaultPagingDRX]{
	Root:       []string{"rf128", "rf256", "rf512", "rf1024"},
	Extensible: true,
}

var enumAIoTSupport = per.Enum[AIoTSupport]{
	Root:       []string{"aIoT-only", "aIoT-and-NRUu"},
	Extensible: true,
}

var enumIABSupported = per.Enum[IABSupported]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumMobileIABSupported = per.Enum[MobileIABSupported]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumNBIoTPagingDRX = per.Enum[NBIoTPagingDRX]{
	Root:       []string{"rf32", "rf64", "rf128", "rf256", "rf512", "rf1024"},
	Extensible: true,
}

var enumPagingCause = per.Enum[PagingCause]{
	Root:       []string{"voice"},
	Extensible: true,
}

var enumLPWUSDisableIndication = per.Enum[LPWUSDisableIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumRRCState = per.Enum[RRCState]{
	Root:       []string{"inactive", "connected"},
	Extensible: true,
}

var enumXrDeviceWith2Rx = per.Enum[XrDeviceWith2Rx]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumMICOModeIndication = per.Enum[MICOModeIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumMicoAllPLMN = per.Enum[MicoAllPLMN]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumPagingCauseIndicationForVoiceService = per.Enum[PagingCauseIndicationForVoiceService]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumCNMTCommunicationHandling = per.Enum[CNMTCommunicationHandling]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumVehicleUE = per.Enum[VehicleUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumPedestrianUE = per.Enum[PedestrianUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeDirectDiscovery = per.Enum[FiveGProSeDirectDiscovery]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeDirectCommunication = per.Enum[FiveGProSeDirectCommunication]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2UEtoNetworkRelay = per.Enum[FiveGProSeLayer2UEtoNetworkRelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer3UEtoNetworkRelay = per.Enum[FiveGProSeLayer3UEtoNetworkRelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2RemoteUE = per.Enum[FiveGProSeLayer2RemoteUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2Multipath = per.Enum[FiveGProSeLayer2Multipath]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2UEtoUERelay = per.Enum[FiveGProSeLayer2UEtoUERelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2UEtoUERemote = per.Enum[FiveGProSeLayer2UEtoUERemote]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2MHUEtoNetworkRelay = per.Enum[FiveGProSeLayer2MHUEtoNetworkRelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2MHIntermediateUEtoNetworkRelay = per.Enum[FiveGProSeLayer2MHIntermediateUEtoNetworkRelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer2MHRemote = per.Enum[FiveGProSeLayer2MHRemote]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumFiveGProSeLayer3MHUEtoNetworkRelay = per.Enum[FiveGProSeLayer3MHUEtoNetworkRelay]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumSLPositioningRangingAuthorized = per.Enum[SLPositioningRangingAuthorized]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumEmergencyFallbackRequestIndicator = per.Enum[EmergencyFallbackRequestIndicator]{
	Root:       []string{"emergency-fallback-requested"},
	Extensible: true,
}

var enumEmergencyServiceTargetCN = per.Enum[EmergencyServiceTargetCN]{
	Root:       []string{"fiveGC", "epc"},
	Extensible: true,
}

var enumNBIoTPagingeDRXCycle = per.Enum[NBIoTPagingeDRXCycle]{
	Root:       []string{"hf2", "hf4", "hf6", "hf8", "hf10", "hf12", "hf14", "hf16", "hf32", "hf64", "hf128", "hf256", "hf512", "hf1024"},
	Extensible: true,
}

var enumNBIoTPagingTimeWindow = per.Enum[NBIoTPagingTimeWindow]{
	Root:       []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12", "s13", "s14", "s15", "s16"},
	Extensible: true,
}

var enumPagingProbabilityInformation = per.Enum[PagingProbabilityInformation]{
	Root:       []string{"p00", "p05", "p10", "p15", "p20", "p25", "p30", "p35", "p40", "p45", "p50", "p55", "p60", "p65", "p70", "p75", "p80", "p85", "p90", "p95", "p100"},
	Extensible: true,
}

var enumEUTRAPagingeDRXCycle = per.Enum[EUTRAPagingeDRXCycle]{
	Root:       []string{"hfhalf", "hf1", "hf2", "hf4", "hf6", "hf8", "hf10", "hf12", "hf14", "hf16", "hf32", "hf64", "hf128", "hf256"},
	Extensible: true,
}

var enumEUTRAPagingTimeWindow = per.Enum[EUTRAPagingTimeWindow]{
	Root:       []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12", "s13", "s14", "s15", "s16"},
	Extensible: true,
}

var enumNRPagingeDRXCycle = per.Enum[NRPagingeDRXCycle]{
	Root:       []string{"hfquarter", "hfhalf", "hf1", "hf2", "hf4", "hf8", "hf16", "hf32", "hf64", "hf128", "hf256", "hf512", "hf1024"},
	Extensible: true,
}

var enumNRPagingTimeWindow = per.Enum[NRPagingTimeWindow]{
	Root:      []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12", "s13", "s14", "s15", "s16"},
	Extension: []string{"s17", "s18", "s19", "s20", "s21", "s22", "s23", "s24", "s25", "s26", "s27", "s28", "s29", "s30", "s31", "s32"},
}

var enumExpectedHOInterval = per.Enum[ExpectedHOInterval]{
	Root:       []string{"sec15", "sec30", "sec60", "sec90", "sec120", "sec180", "long-time"},
	Extensible: true,
}

var enumExpectedUEMobility = per.Enum[ExpectedUEMobility]{
	Root:       []string{"stationary", "mobile"},
	Extensible: true,
}

var enumA2XPC5QoSFlowItemA2XRange = per.Enum[A2XPC5QoSFlowItemA2XRange]{
	Root:       []string{"m50", "m80", "m180", "m200", "m350", "m400", "m500", "m700", "m1000"},
	Extensible: true,
}

var enumNextPagingAreaScope = per.Enum[NextPagingAreaScope]{
	Root:       []string{"same", "changed"},
	Extensible: true,
}

var enumMBSAssistanceInformation = per.Enum[MBSAssistanceInformation]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumRangeType = per.Enum[RangeType]{
	Root:       []string{"m50", "m80", "m180", "m200", "m350", "m400", "m500", "m700", "m1000"},
	Extensible: true,
}

var enumClockQualityAcceptanceCriteriaTraceabletoUTC = per.Enum[ClockQualityAcceptanceCriteriaTraceabletoUTC]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumClockQualityAcceptanceCriteriaTraceabletoGNSS = per.Enum[ClockQualityAcceptanceCriteriaTraceabletoGNSS]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumIntersystemUnnecessaryHOEarlyIRATHO = per.Enum[IntersystemUnnecessaryHOEarlyIRATHO]{
	Root:       []string{"true", "false"},
	Extensible: true,
}

var enumNotificationCellItemNotifyFlag = per.Enum[NotificationCellItemNotifyFlag]{
	Root:       []string{"activated", "deactivated"},
	Extensible: true,
}

var enumNumberOfMeasurementReportingLevels = per.Enum[NumberOfMeasurementReportingLevels]{
	Root:      []string{"n2", "n3", "n4", "n5", "n10"},
	Extension: []string{"n0"},
}

var enumReportingPeriodicity = per.Enum[ReportingPeriodicity]{
	Root:       []string{"stop", "single", "ms1000", "ms2000", "ms5000", "ms10000"},
	Extensible: true,
}
