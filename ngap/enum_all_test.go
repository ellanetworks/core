// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func moreExtensibleEnums() []enumCase {
	return []enumCase{
		enumCaseOf("TypeOfError", &enumTypeOfError, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TypeOfError(v)
		}),
		enumCaseOf("UEPresence", &enumUEPresence, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEPresence(v)
		}),
		enumCaseOf("DelayCritical", &enumDelayCritical, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DelayCritical(v)
		}),
		enumCaseOf("ReflectiveQosAttribute", &enumReflectiveQosAttribute, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ReflectiveQosAttribute(v)
		}),
		enumCaseOf("AdditionalQosFlowInformation", &enumAdditionalQosFlowInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AdditionalQosFlowInformation(v)
		}),
		enumCaseOf("PreemptionCapability", &enumPreemptionCapability, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PreemptionCapability(v)
		}),
		enumCaseOf("PreemptionVulnerability", &enumPreemptionVulnerability, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PreemptionVulnerability(v)
		}),
		enumCaseOf("NotificationControl", &enumNotificationControl, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NotificationControl(v)
		}),
		enumCaseOf("QosFlowMappingIndication", &enumQosFlowMappingIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return QosFlowMappingIndication(v)
		}),
		enumCaseOf("DataForwardingAccepted", &enumDataForwardingAccepted, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DataForwardingAccepted(v)
		}),
		enumCaseOf("IntegrityProtectionIndication", &enumIntegrityProtectionIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IntegrityProtectionIndication(v)
		}),
		enumCaseOf("ConfidentialityProtectionIndication", &enumConfidentialityProtectionIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ConfidentialityProtectionIndication(v)
		}),
		enumCaseOf("MaximumIntegrityProtectedDataRate", &enumMaximumIntegrityProtectedDataRate, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MaximumIntegrityProtectedDataRate(v)
		}),
		enumCaseOf("IntegrityProtectionResult", &enumIntegrityProtectionResult, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IntegrityProtectionResult(v)
		}),
		enumCaseOf("ConfidentialityProtectionResult", &enumConfidentialityProtectionResult, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ConfidentialityProtectionResult(v)
		}),
		enumCaseOf("DirectForwardingPathAvailability", &enumDirectForwardingPathAvailability, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DirectForwardingPathAvailability(v)
		}),
		enumCaseOf("DLNGUTNLInformationReused", &enumDLNGUTNLInformationReused, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DLNGUTNLInformationReused(v)
		}),
		enumCaseOf("TimerApproachForGUAMIRemoval", &enumTimerApproachForGUAMIRemoval, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TimerApproachForGUAMIRemoval(v)
		}),
		enumCaseOf("ConcurrentWarningMessageInd", &enumConcurrentWarningMessageInd, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ConcurrentWarningMessageInd(v)
		}),
		enumCaseOf("CancelAllWarningMessages", &enumCancelAllWarningMessages, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CancelAllWarningMessages(v)
		}),
		enumCaseOf("ReportArea", &enumReportArea, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ReportArea(v)
		}),
		enumCaseOf("UEContextRequest", &enumUEContextRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEContextRequest(v)
		}),
		enumCaseOf("GWContextReleaseIndication", &enumGWContextReleaseIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return GWContextReleaseIndication(v)
		}),
		enumCaseOf("TNLAssociationUsage", &enumTNLAssociationUsage, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TNLAssociationUsage(v)
		}),
		enumCaseOf("PagingPriority", &enumPagingPriority, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingPriority(v)
		}),
		enumCaseOf("PagingOrigin", &enumPagingOrigin, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingOrigin(v)
		}),
		enumCaseOf("OnboardingSupport", &enumOnboardingSupport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return OnboardingSupport(v)
		}),
		enumCaseOf("NotifySourceNGRANNode", &enumNotifySourceNGRANNode, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NotifySourceNGRANNode(v)
		}),
		enumCaseOf("OverloadAction", &enumOverloadAction, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return OverloadAction(v)
		}),
		enumCaseOf("UERetentionInformation", &enumUERetentionInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UERetentionInformation(v)
		}),
		enumCaseOf("NewSecurityContextInd", &enumNewSecurityContextInd, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NewSecurityContextInd(v)
		}),
		enumCaseOf("PDUSessionType", &enumPDUSessionType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PDUSessionType(v)
		}),
		enumCaseOf("DLForwarding", &enumDLForwarding, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DLForwarding(v)
		}),
		enumCaseOf("CellSize", &enumCellSize, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CellSize(v)
		}),
		enumCaseOf("DataForwardingNotPossible", &enumDataForwardingNotPossible, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DataForwardingNotPossible(v)
		}),
		enumCaseOf("UserPlaneFailureType", &enumUserPlaneFailureType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UserPlaneFailureType(v)
		}),
		enumCaseOf("ResetAll", &enumResetAll, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ResetAll(v)
		}),
		enumCaseOf("LineType", &enumLineType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LineType(v)
		}),
		enumCaseOf("ConfiguredTACIndication", &enumConfiguredTACIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ConfiguredTACIndication(v)
		}),
		enumCaseOf("RATInformation", &enumRATInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RATInformation(v)
		}),
		enumCaseOf("LocationReportingAdditionalInfo", &enumLocationReportingAdditionalInfo, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LocationReportingAdditionalInfo(v)
		}),
		enumCaseOf("AerialUEReportingPeriodicity", &enumAerialUEReportingPeriodicity, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AerialUEReportingPeriodicity(v)
		}),
		enumCaseOf("MBSSupportIndicator", &enumMBSSupportIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MBSSupportIndicator(v)
		}),
		enumCaseOf("PDUSetbasedHandlingIndicator", &enumPDUSetbasedHandlingIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PDUSetbasedHandlingIndicator(v)
		}),
		enumCaseOf("CNTypeRestrictionsForServing", &enumCNTypeRestrictionsForServing, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CNTypeRestrictionsForServing(v)
		}),
		enumCaseOf("QosMonitoringRequest", &enumQosMonitoringRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return QosMonitoringRequest(v)
		}),
		enumCaseOf("DLPDUSetInformationMarkingSupportIndication", &enumDLPDUSetInformationMarkingSupportIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DLPDUSetInformationMarkingSupportIndication(v)
		}),
		enumCaseOf("IndicationofBitrateAdaptation", &enumIndicationofBitrateAdaptation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IndicationofBitrateAdaptation(v)
		}),
		enumCaseOf("UserPlaneFailureIndicationReport", &enumUserPlaneFailureIndicationReport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UserPlaneFailureIndicationReport(v)
		}),
		enumCaseOf("ULForwarding", &enumULForwarding, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ULForwarding(v)
		}),
		enumCaseOf("DAPSRequestInfoDAPSIndicator", &enumDAPSRequestInfoDAPSIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return DAPSRequestInfoDAPSIndicator(v)
		}),
		enumCaseOf("RSN", &enumRSN, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RSN(v)
		}),
		enumCaseOf("CNTypeRestrictionsForEquivalentItemCnType", &enumCNTypeRestrictionsForEquivalentItemCnType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CNTypeRestrictionsForEquivalentItemCnType(v)
		}),
		enumCaseOf("SourceOfUEActivityBehaviourInformation", &enumSourceOfUEActivityBehaviourInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SourceOfUEActivityBehaviourInformation(v)
		}),
		enumCaseOf("PDUsetQoSInformationPduSetIntegratedHandlingInformation", &enumPDUsetQoSInformationPduSetIntegratedHandlingInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PDUsetQoSInformationPduSetIntegratedHandlingInformation(v)
		}),
		enumCaseOf("MonitoringRequest", &enumMonitoringRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MonitoringRequest(v)
		}),
		enumCaseOf("ECNMarkingAtRANRequest", &enumECNMarkingAtRANRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ECNMarkingAtRANRequest(v)
		}),
		enumCaseOf("ECNMarkingAtUPFRequest", &enumECNMarkingAtUPFRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ECNMarkingAtUPFRequest(v)
		}),
		enumCaseOf("CongestionInformationRequest", &enumCongestionInformationRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CongestionInformationRequest(v)
		}),
		enumCaseOf("PDUSessionUsageReportRATType", &enumPDUSessionUsageReportRATType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PDUSessionUsageReportRATType(v)
		}),
		enumCaseOf("MNonlyMDTcollection", &enumMNonlyMDTcollection, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MNonlyMDTcollection(v)
		}),
		enumCaseOf("ActivationStatus", &enumActivationStatus, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ActivationStatus(v)
		}),
		enumCaseOf("QoSFlowsUsageReportItemRATType", &enumQoSFlowsUsageReportItemRATType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return QoSFlowsUsageReportItemRATType(v)
		}),
		enumCaseOf("MDTActivation", &enumMDTActivation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MDTActivation(v)
		}),
		enumCaseOf("AllowedPNINPNItemPNINPNrestricted", &enumAllowedPNINPNItemPNINPNrestricted, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AllowedPNINPNItemPNINPNrestricted(v)
		}),
		enumCaseOf("RANfeedbacktypereactiveCapabilityForBATAdaptation", &enumRANfeedbacktypereactiveCapabilityForBATAdaptation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RANfeedbacktypereactiveCapabilityForBATAdaptation(v)
		}),
		enumCaseOf("UEAppLayerMeasConfigInfoQoEMeasurementStatus", &enumUEAppLayerMeasConfigInfoQoEMeasurementStatus, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEAppLayerMeasConfigInfoQoEMeasurementStatus(v)
		}),
		enumCaseOf("BluetoothMeasurementConfigurationBtrssi", &enumBluetoothMeasurementConfigurationBtrssi, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return BluetoothMeasurementConfigurationBtrssi(v)
		}),
		enumCaseOf("WLANMeasurementConfigurationWlanrssi", &enumWLANMeasurementConfigurationWlanrssi, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return WLANMeasurementConfigurationWlanrssi(v)
		}),
		enumCaseOf("WLANMeasurementConfigurationWlanrtt", &enumWLANMeasurementConfigurationWlanrtt, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return WLANMeasurementConfigurationWlanrtt(v)
		}),
		enumCaseOf("LoggingInterval", &enumLoggingInterval, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LoggingInterval(v)
		}),
		enumCaseOf("LoggingDuration", &enumLoggingDuration, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LoggingDuration(v)
		}),
		enumCaseOf("EarlyMeasurement", &enumEarlyMeasurement, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EarlyMeasurement(v)
		}),
		enumCaseOf("ServiceType", &enumServiceType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ServiceType(v)
		}),
		enumCaseOf("AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList", &enumAvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList(v)
		}),
		enumCaseOf("AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup", &enumAvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup(v)
		}),
		enumCaseOf("CommServiceType", &enumCommServiceType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CommServiceType(v)
		}),
		enumCaseOf("QoERVQoEReportingPathsQoEReportingPath", &enumQoERVQoEReportingPathsQoEReportingPath, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return QoERVQoEReportingPathsQoEReportingPath(v)
		}),
		enumCaseOf("QoERVQoEReportingPathsRVQoEReportingPath", &enumQoERVQoEReportingPathsRVQoEReportingPath, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return QoERVQoEReportingPathsRVQoEReportingPath(v)
		}),
		enumCaseOf("M1ReportingTrigger", &enumM1ReportingTrigger, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M1ReportingTrigger(v)
		}),
		enumCaseOf("IncludeBeamMeasurementsIndication", &enumIncludeBeamMeasurementsIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IncludeBeamMeasurementsIndication(v)
		}),
		enumCaseOf("M4period", &enumM4period, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M4period(v)
		}),
		enumCaseOf("Linkstolog", &enumLinkstolog, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return Linkstolog(v)
		}),
		enumCaseOf("M4ReportAmountMDT", &enumM4ReportAmountMDT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M4ReportAmountMDT(v)
		}),
		enumCaseOf("M5period", &enumM5period, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M5period(v)
		}),
		enumCaseOf("M5ReportAmountMDT", &enumM5ReportAmountMDT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M5ReportAmountMDT(v)
		}),
		enumCaseOf("M6reportInterval", &enumM6reportInterval, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M6reportInterval(v)
		}),
		enumCaseOf("M6ReportAmountMDT", &enumM6ReportAmountMDT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M6ReportAmountMDT(v)
		}),
		enumCaseOf("M7ReportAmountMDT", &enumM7ReportAmountMDT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return M7ReportAmountMDT(v)
		}),
		enumCaseOf("BluetoothMeasConfig", &enumBluetoothMeasConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return BluetoothMeasConfig(v)
		}),
		enumCaseOf("WLANMeasConfig", &enumWLANMeasConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return WLANMeasConfig(v)
		}),
		enumCaseOf("SensorMeasConfig", &enumSensorMeasConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SensorMeasConfig(v)
		}),
		enumCaseOf("EventTriggerOutOfCoverage", &enumEventTriggerOutOfCoverage, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EventTriggerOutOfCoverage(v)
		}),
		enumCaseOf("ExtendedReportIntervalMDT", &enumExtendedReportIntervalMDT, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ExtendedReportIntervalMDT(v)
		}),
		enumCaseOf("BeamMeasurementsReportQuantityRSRP", &enumBeamMeasurementsReportQuantityRSRP, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return BeamMeasurementsReportQuantityRSRP(v)
		}),
		enumCaseOf("BeamMeasurementsReportQuantityRSRQ", &enumBeamMeasurementsReportQuantityRSRQ, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return BeamMeasurementsReportQuantityRSRQ(v)
		}),
		enumCaseOf("BeamMeasurementsReportQuantitySINR", &enumBeamMeasurementsReportQuantitySINR, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return BeamMeasurementsReportQuantitySINR(v)
		}),
		enumCaseOf("ExcessPacketDelayThresholdValue", &enumExcessPacketDelayThresholdValue, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ExcessPacketDelayThresholdValue(v)
		}),
		enumCaseOf("SensorNameConfigUncompensatedBarometricConfig", &enumSensorNameConfigUncompensatedBarometricConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SensorNameConfigUncompensatedBarometricConfig(v)
		}),
		enumCaseOf("SensorNameConfigUeSpeedConfig", &enumSensorNameConfigUeSpeedConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SensorNameConfigUeSpeedConfig(v)
		}),
		enumCaseOf("SensorNameConfigUeOrientationConfig", &enumSensorNameConfigUeOrientationConfig, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SensorNameConfigUeOrientationConfig(v)
		}),
		enumCaseOf("RRCInactiveTransitionReportRequest", &enumRRCInactiveTransitionReportRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RRCInactiveTransitionReportRequest(v)
		}),
		enumCaseOf("RedirectionVoiceFallback", &enumRedirectionVoiceFallback, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RedirectionVoiceFallback(v)
		}),
		enumCaseOf("SRVCCOperationPossible", &enumSRVCCOperationPossible, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SRVCCOperationPossible(v)
		}),
		enumCaseOf("IABAuthorized", &enumIABAuthorized, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IABAuthorized(v)
		}),
		enumCaseOf("EnhancedCoverageRestriction", &enumEnhancedCoverageRestriction, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EnhancedCoverageRestriction(v)
		}),
		enumCaseOf("UEDifferentiationInfoPeriodicCommunicationIndicator", &enumUEDifferentiationInfoPeriodicCommunicationIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEDifferentiationInfoPeriodicCommunicationIndicator(v)
		}),
		enumCaseOf("UEDifferentiationInfoStationaryIndication", &enumUEDifferentiationInfoStationaryIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEDifferentiationInfoStationaryIndication(v)
		}),
		enumCaseOf("UEDifferentiationInfoTrafficProfile", &enumUEDifferentiationInfoTrafficProfile, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEDifferentiationInfoTrafficProfile(v)
		}),
		enumCaseOf("UEDifferentiationInfoBatteryIndication", &enumUEDifferentiationInfoBatteryIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEDifferentiationInfoBatteryIndication(v)
		}),
		enumCaseOf("CEmodeBrestricted", &enumCEmodeBrestricted, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CEmodeBrestricted(v)
		}),
		enumCaseOf("UEUPCIoTSupport", &enumUEUPCIoTSupport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UEUPCIoTSupport(v)
		}),
		enumCaseOf("TimeSyncAssistanceInfoTimeDistributionIndication", &enumTimeSyncAssistanceInfoTimeDistributionIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return TimeSyncAssistanceInfoTimeDistributionIndication(v)
		}),
		enumCaseOf("AerialUEsubscriptionInformation", &enumAerialUEsubscriptionInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AerialUEsubscriptionInformation(v)
		}),
		enumCaseOf("NRA2XServicesAuthorizedAerialUE", &enumNRA2XServicesAuthorizedAerialUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRA2XServicesAuthorizedAerialUE(v)
		}),
		enumCaseOf("NRA2XServicesAuthorizedAerialControllerUE", &enumNRA2XServicesAuthorizedAerialControllerUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRA2XServicesAuthorizedAerialControllerUE(v)
		}),
		enumCaseOf("LTEA2XServicesAuthorizedAerialUE", &enumLTEA2XServicesAuthorizedAerialUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LTEA2XServicesAuthorizedAerialUE(v)
		}),
		enumCaseOf("LTEA2XServicesAuthorizedAerialControllerUE", &enumLTEA2XServicesAuthorizedAerialControllerUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LTEA2XServicesAuthorizedAerialControllerUE(v)
		}),
		enumCaseOf("MobileIABAuthorized", &enumMobileIABAuthorized, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MobileIABAuthorized(v)
		}),
		enumCaseOf("NoPDUSessionIndication", &enumNoPDUSessionIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NoPDUSessionIndication(v)
		}),
		enumCaseOf("RedCapIndication", &enumRedCapIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RedCapIndication(v)
		}),
		enumCaseOf("ERedCapIndication", &enumERedCapIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ERedCapIndication(v)
		}),
		enumCaseOf("NetworkControlledRepeaterAuthorized", &enumNetworkControlledRepeaterAuthorized, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NetworkControlledRepeaterAuthorized(v)
		}),
		enumCaseOf("IABNodeIndication", &enumIABNodeIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IABNodeIndication(v)
		}),
		enumCaseOf("CEmodeBSupportIndicator", &enumCEmodeBSupportIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CEmodeBSupportIndicator(v)
		}),
		enumCaseOf("LTEMIndication", &enumLTEMIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LTEMIndication(v)
		}),
		enumCaseOf("EDTSession", &enumEDTSession, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EDTSession(v)
		}),
		enumCaseOf("AuthenticatedIndication", &enumAuthenticatedIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AuthenticatedIndication(v)
		}),
		enumCaseOf("MobileIABNodeIndication", &enumMobileIABNodeIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MobileIABNodeIndication(v)
		}),
		enumCaseOf("AUN3DeviceAccessInfoAUN3DeviceAccess", &enumAUN3DeviceAccessInfoAUN3DeviceAccess, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AUN3DeviceAccessInfoAUN3DeviceAccess(v)
		}),
		enumCaseOf("AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation", &enumAUN3DeviceAccessInfoExistingNGConnectionwithSameLocation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation(v)
		}),
		enumCaseOf("UECapabilityInfoRequest", &enumUECapabilityInfoRequest, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return UECapabilityInfoRequest(v)
		}),
		enumCaseOf("EndIndication", &enumEndIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EndIndication(v)
		}),
		enumCaseOf("NBIoTDefaultPagingDRX", &enumNBIoTDefaultPagingDRX, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NBIoTDefaultPagingDRX(v)
		}),
		enumCaseOf("AIoTSupport", &enumAIoTSupport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return AIoTSupport(v)
		}),
		enumCaseOf("IABSupported", &enumIABSupported, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IABSupported(v)
		}),
		enumCaseOf("MobileIABSupported", &enumMobileIABSupported, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MobileIABSupported(v)
		}),
		enumCaseOf("NBIoTPagingDRX", &enumNBIoTPagingDRX, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NBIoTPagingDRX(v)
		}),
		enumCaseOf("PagingCause", &enumPagingCause, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingCause(v)
		}),
		enumCaseOf("LPWUSDisableIndication", &enumLPWUSDisableIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return LPWUSDisableIndication(v)
		}),
		enumCaseOf("RRCState", &enumRRCState, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RRCState(v)
		}),
		enumCaseOf("XrDeviceWith2Rx", &enumXrDeviceWith2Rx, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return XrDeviceWith2Rx(v)
		}),
		enumCaseOf("MICOModeIndication", &enumMICOModeIndication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MICOModeIndication(v)
		}),
		enumCaseOf("MicoAllPLMN", &enumMicoAllPLMN, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MicoAllPLMN(v)
		}),
		enumCaseOf("PagingCauseIndicationForVoiceService", &enumPagingCauseIndicationForVoiceService, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingCauseIndicationForVoiceService(v)
		}),
		enumCaseOf("CNMTCommunicationHandling", &enumCNMTCommunicationHandling, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CNMTCommunicationHandling(v)
		}),
		enumCaseOf("VehicleUE", &enumVehicleUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return VehicleUE(v)
		}),
		enumCaseOf("PedestrianUE", &enumPedestrianUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PedestrianUE(v)
		}),
		enumCaseOf("FiveGProSeDirectDiscovery", &enumFiveGProSeDirectDiscovery, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeDirectDiscovery(v)
		}),
		enumCaseOf("FiveGProSeDirectCommunication", &enumFiveGProSeDirectCommunication, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeDirectCommunication(v)
		}),
		enumCaseOf("FiveGProSeLayer2UEtoNetworkRelay", &enumFiveGProSeLayer2UEtoNetworkRelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2UEtoNetworkRelay(v)
		}),
		enumCaseOf("FiveGProSeLayer3UEtoNetworkRelay", &enumFiveGProSeLayer3UEtoNetworkRelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer3UEtoNetworkRelay(v)
		}),
		enumCaseOf("FiveGProSeLayer2RemoteUE", &enumFiveGProSeLayer2RemoteUE, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2RemoteUE(v)
		}),
		enumCaseOf("FiveGProSeLayer2Multipath", &enumFiveGProSeLayer2Multipath, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2Multipath(v)
		}),
		enumCaseOf("FiveGProSeLayer2UEtoUERelay", &enumFiveGProSeLayer2UEtoUERelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2UEtoUERelay(v)
		}),
		enumCaseOf("FiveGProSeLayer2UEtoUERemote", &enumFiveGProSeLayer2UEtoUERemote, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2UEtoUERemote(v)
		}),
		enumCaseOf("FiveGProSeLayer2MHUEtoNetworkRelay", &enumFiveGProSeLayer2MHUEtoNetworkRelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2MHUEtoNetworkRelay(v)
		}),
		enumCaseOf("FiveGProSeLayer2MHIntermediateUEtoNetworkRelay", &enumFiveGProSeLayer2MHIntermediateUEtoNetworkRelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2MHIntermediateUEtoNetworkRelay(v)
		}),
		enumCaseOf("FiveGProSeLayer2MHRemote", &enumFiveGProSeLayer2MHRemote, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer2MHRemote(v)
		}),
		enumCaseOf("FiveGProSeLayer3MHUEtoNetworkRelay", &enumFiveGProSeLayer3MHUEtoNetworkRelay, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return FiveGProSeLayer3MHUEtoNetworkRelay(v)
		}),
		enumCaseOf("SLPositioningRangingAuthorized", &enumSLPositioningRangingAuthorized, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return SLPositioningRangingAuthorized(v)
		}),
		enumCaseOf("EmergencyFallbackRequestIndicator", &enumEmergencyFallbackRequestIndicator, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EmergencyFallbackRequestIndicator(v)
		}),
		enumCaseOf("EmergencyServiceTargetCN", &enumEmergencyServiceTargetCN, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EmergencyServiceTargetCN(v)
		}),
		enumCaseOf("NBIoTPagingeDRXCycle", &enumNBIoTPagingeDRXCycle, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NBIoTPagingeDRXCycle(v)
		}),
		enumCaseOf("NBIoTPagingTimeWindow", &enumNBIoTPagingTimeWindow, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NBIoTPagingTimeWindow(v)
		}),
		enumCaseOf("PagingProbabilityInformation", &enumPagingProbabilityInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return PagingProbabilityInformation(v)
		}),
		enumCaseOf("EUTRAPagingeDRXCycle", &enumEUTRAPagingeDRXCycle, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EUTRAPagingeDRXCycle(v)
		}),
		enumCaseOf("EUTRAPagingTimeWindow", &enumEUTRAPagingTimeWindow, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return EUTRAPagingTimeWindow(v)
		}),
		enumCaseOf("NRPagingeDRXCycle", &enumNRPagingeDRXCycle, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRPagingeDRXCycle(v)
		}),
		enumCaseOf("NRPagingTimeWindow", &enumNRPagingTimeWindow, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NRPagingTimeWindow(v)
		}),
		enumCaseOf("ExpectedHOInterval", &enumExpectedHOInterval, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ExpectedHOInterval(v)
		}),
		enumCaseOf("ExpectedUEMobility", &enumExpectedUEMobility, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ExpectedUEMobility(v)
		}),
		enumCaseOf("A2XPC5QoSFlowItemA2XRange", &enumA2XPC5QoSFlowItemA2XRange, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return A2XPC5QoSFlowItemA2XRange(v)
		}),
		enumCaseOf("NextPagingAreaScope", &enumNextPagingAreaScope, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NextPagingAreaScope(v)
		}),
		enumCaseOf("MBSAssistanceInformation", &enumMBSAssistanceInformation, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return MBSAssistanceInformation(v)
		}),
		enumCaseOf("RangeType", &enumRangeType, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return RangeType(v)
		}),
		enumCaseOf("ClockQualityAcceptanceCriteriaTraceabletoUTC", &enumClockQualityAcceptanceCriteriaTraceabletoUTC, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ClockQualityAcceptanceCriteriaTraceabletoUTC(v)
		}),
		enumCaseOf("ClockQualityAcceptanceCriteriaTraceabletoGNSS", &enumClockQualityAcceptanceCriteriaTraceabletoGNSS, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ClockQualityAcceptanceCriteriaTraceabletoGNSS(v)
		}),
		enumCaseOf("IntersystemUnnecessaryHOEarlyIRATHO", &enumIntersystemUnnecessaryHOEarlyIRATHO, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return IntersystemUnnecessaryHOEarlyIRATHO(v)
		}),
		enumCaseOf("NotificationCellItemNotifyFlag", &enumNotificationCellItemNotifyFlag, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NotificationCellItemNotifyFlag(v)
		}),
		enumCaseOf("NumberOfMeasurementReportingLevels", &enumNumberOfMeasurementReportingLevels, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return NumberOfMeasurementReportingLevels(v)
		}),
		enumCaseOf("ReportingPeriodicity", &enumReportingPeriodicity, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return ReportingPeriodicity(v)
		}),
		enumCaseOf("CauseRadioNetwork", &enumCauseRadioNetwork, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseRadioNetwork(v)
		}),
		enumCaseOf("CauseTransport", &enumCauseTransport, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseTransport(v)
		}),
		enumCaseOf("CauseNas", &enumCauseNas, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseNas(v)
		}),
		enumCaseOf("CauseProtocol", &enumCauseProtocol, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseProtocol(v)
		}),
		enumCaseOf("CauseMisc", &enumCauseMisc, func(v int64) interface {
			per.Marshaler
			Name() string
			Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
		} {
			return CauseMisc(v)
		}),
	}
}
