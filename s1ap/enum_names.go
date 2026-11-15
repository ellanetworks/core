// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

// ASN.1 enumeration item names from TS 36.413 §9.3.4 and §9.3.5. A Name backed
// by a map returns the empty string for a value the module does not name; one
// generated from a per.Enum returns "unknown(n)".

var enumTriggeringMessage = per.Enum[TriggeringMessage]{
	Root: []string{"initiating-message", "successful-outcome", "unsuccessfull-outcome"}, //nolint:misspell
}

var enumHandoverType = per.Enum[HandoverType]{
	Root:      []string{"intralte", "ltetoutran", "ltetogeran", "utrantolte", "gerantolte"},
	Extension: []string{"eps-to-5gs", "fivegs-to-eps"},
}

var enumCNDomain = per.Enum[CNDomain]{
	Root: []string{"ps", "cs"},
}

var enumPreemptionCapability = per.Enum[PreemptionCapability]{
	Root: []string{"shall-not-trigger-pre-emption", "may-trigger-pre-emption"},
}

var enumPreemptionVulnerability = per.Enum[PreemptionVulnerability]{
	Root: []string{"not-pre-emptable", "pre-emptable"},
}

var enumTypeOfError = per.Enum[TypeOfError]{
	Root:       []string{"not-understood", "missing"},
	Extensible: true,
}

var enumEventType = per.Enum[EventType]{
	Root:       []string{"direct", "change-of-serve-cell", "stop-change-of-serve-cell"},
	Extensible: true,
}

var enumReportArea = per.Enum[ReportArea]{
	Root:       []string{"ecgi"},
	Extensible: true,
}

var enumBearerType = per.Enum[BearerType]{
	Root:       []string{"non-IP"},
	Extensible: true,
}

var enumEthernetType = per.Enum[EthernetType]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumIntegrityProtectionIndication = per.Enum[IntegrityProtectionIndication]{
	Root:       []string{"required", "preferred", "not-needed"},
	Extensible: true,
}

var enumPagingPriority = per.Enum[PagingPriority]{
	Root:       []string{"priolevel1", "priolevel2", "priolevel3", "priolevel4", "priolevel5", "priolevel6", "priolevel7", "priolevel8"},
	Extensible: true,
}

var enumUERetentionInformation = per.Enum[UERetentionInformation]{
	Root:       []string{"ues-retained"},
	Extensible: true,
}

var enumPagingDRX = per.Enum[PagingDRX]{
	Root:       []string{"v32", "v64", "v128", "v256"},
	Extensible: true,
}

var enumForbiddenInterRATs = per.Enum[ForbiddenInterRATs]{
	Root:      []string{"all", "geran", "utran", "cdma2000"},
	Extension: []string{"geranandutran", "cdma2000andutran"},
}

var enumDataForwardingNotPossible = per.Enum[DataForwardingNotPossible]{
	Root:       []string{"data-Forwarding-not-Possible"},
	Extensible: true,
}

var enumTimeToWait = per.Enum[TimeToWait]{
	Root:       []string{"v1s", "v2s", "v5s", "v10s", "v20s", "v60s"},
	Extensible: true,
}

var enumTraceDepth = per.Enum[TraceDepth]{
	Root:       []string{"minimum", "medium", "maximum", "minimumWithoutVendorSpecificExtension", "mediumWithoutVendorSpecificExtension", "maximumWithoutVendorSpecificExtension"},
	Extensible: true,
}

var enumRRCEstablishmentCause = per.Enum[RRCEstablishmentCause]{
	Root:      []string{"emergency", "highPriorityAccess", "mt-Access", "mo-Signalling", "mo-Data"},
	Extension: []string{"delay-TolerantAccess", "mo-VoiceCall", "mo-ExceptionData"},
}

var enumGWContextReleaseIndication = per.Enum[GWContextReleaseIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumDLForwarding = per.Enum[DLForwarding]{
	Root:       []string{"dL-Forwarding-proposed"},
	Extensible: true,
}

var enumCellSize = per.Enum[CellSize]{
	Root:       []string{"verysmall", "small", "medium", "large"},
	Extensible: true,
}

var enumDirectForwardingPathAvailability = per.Enum[DirectForwardingPathAvailability]{
	Root:       []string{"directPathAvailable"},
	Extensible: true,
}

var enumResetAll = per.Enum[ResetAll]{
	Root:       []string{"reset-all"},
	Extensible: true,
}

var enumOverloadAction = per.Enum[OverloadAction]{
	Root:      []string{"reject-non-emergency-mo-dt", "reject-rrc-cr-signalling", "permit-emergency-sessions-and-mobile-terminated-services-only"},
	Extension: []string{"permit-high-priority-sessions-and-mobile-terminated-services-only", "reject-delay-tolerant-access", "permit-high-priority-sessions-and-exception-reporting-and-mobile-terminated-services-only", "not-accept-mo-data-or-delay-tolerant-access-from-CP-CIoT"},
}

var enumGUMMEIType = per.Enum[GUMMEIType]{
	Root:      []string{"native", "mapped"},
	Extension: []string{"mappedFrom5G"},
}

var enumNRrestrictioninEPSasSecondaryRAT = per.Enum[NRrestrictioninEPSasSecondaryRAT]{
	Root:       []string{"nRrestrictedinEPSasSecondaryRAT"},
	Extensible: true,
}

var enumUnlicensedSpectrumRestriction = per.Enum[UnlicensedSpectrumRestriction]{
	Root:       []string{"unlicensed-restricted"},
	Extensible: true,
}

var enumNRrestrictionin5GS = per.Enum[NRrestrictionin5GS]{
	Root:       []string{"nRrestrictedin5GS"},
	Extensible: true,
}

var enumCNType = per.Enum[CNType]{
	Root:      []string{"fiveGCForbidden"},
	Extension: []string{"epc-Forbiddden"},
}

var enumRequestTypeAdditionalInfo = per.Enum[RequestTypeAdditionalInfo]{
	Root:       []string{"includePSCell"},
	Extensible: true,
}

var enumDAPSIndicator = per.Enum[DAPSIndicator]{
	Root:       []string{"dAPS-HO-required"},
	Extensible: true,
}

var enumIMSvoiceEPSfallbackfrom5G = per.Enum[IMSvoiceEPSfallbackfrom5G]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumEmergencyIndicator = per.Enum[EmergencyIndicator]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumSubcarrierSpacingSSB = per.Enum[SubcarrierSpacingSSB]{
	Root:      []string{"kHz15", "kHz30", "kHz60", "kHz120", "kHz240"},
	Extension: []string{"kHz480", "kHz960"},
}

var enumRATType = per.Enum[RATType]{
	Root:      []string{"nbiot"},
	Extension: []string{"nbiot-leo", "nbiot-meo", "nbiot-geo", "nbiot-othersat", "eutran-leo", "eutran-meo", "eutran-geo", "eutran-othersat"},
}

var enumCoarseUELocationRequested = per.Enum[CoarseUELocationRequested]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumTimeRefDistribution = per.Enum[TimeRefDistribution]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumSONInformationRequest = per.Enum[SONInformationRequest]{
	Root:      []string{"x2TNL-Configuration-Info"},
	Extension: []string{"time-Synchronisation-Info", "activate-Muting", "deactivate-Muting"},
}

var enumSynchronisationStatus = per.Enum[SynchronisationStatus]{
	Root:       []string{"synchronous", "asynchronous"},
	Extensible: true,
}

var enumMutingAvailabilityIndication = per.Enum[MutingAvailabilityIndication]{
	Root:       []string{"available", "unavailable"},
	Extensible: true,
}

var enumMutingPatternPeriod = per.Enum[MutingPatternPeriod]{
	Root:       []string{"ms0", "ms1280", "ms2560", "ms5120", "ms10240"},
	Extensible: true,
}

var enumPatternPeriod = per.Enum[PatternPeriod]{
	Root:       []string{"ms1280", "ms2560", "ms5120", "ms10240"},
	Extensible: true,
}

var enumConcurrentWarningMessageIndicator = per.Enum[ConcurrentWarningMessageIndicator]{
	Root: []string{"true"},
}

var enumKillAllWarningMessages = per.Enum[KillAllWarningMessages]{
	Root: []string{"true"},
}

var enumAdditionalCSFallbackIndicator = per.Enum[AdditionalCSFallbackIndicator]{
	Root:       []string{"no-restriction", "restriction"},
	Extensible: true,
}

var enumAerialUEsubscriptionInformation = per.Enum[AerialUEsubscriptionInformation]{
	Root:       []string{"allowed", "not-allowed"},
	Extensible: true,
}

var enumCEModeBRestricted = per.Enum[CEModeBRestricted]{
	Root:       []string{"restricted", "not-restricted"},
	Extensible: true,
}

var enumCEmodeBSupportIndicator = per.Enum[CEmodeBSupportIndicator]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumCSFallbackIndicator = per.Enum[CSFallbackIndicator]{
	Root:      []string{"cs-fallback-required"},
	Extension: []string{"cs-fallback-high-priority"},
}

var enumCSGMembershipStatus = per.Enum[CSGMembershipStatus]{
	Root: []string{"member", "not-member"},
}

var enumCellAccessMode = per.Enum[CellAccessMode]{
	Root:       []string{"hybrid"},
	Extensible: true,
}

var enumCoverageLevel = per.Enum[CoverageLevel]{
	Root:       []string{"extendedcoverage"},
	Extensible: true,
}

var enumDLNASPDUDeliveryAckRequest = per.Enum[DLNASPDUDeliveryAckRequest]{
	Root:       []string{"requested"},
	Extensible: true,
}

var enumEDTSession = per.Enum[EDTSession]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumEndIndication = per.Enum[EndIndication]{
	Root:       []string{"no-further-data", "further-data-exists"},
	Extensible: true,
}

var enumEnhancedCoverageRestricted = per.Enum[EnhancedCoverageRestricted]{
	Root:       []string{"restricted"},
	Extensible: true,
}

var enumIABAuthorized = per.Enum[IABAuthorized]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumIABNodeIndication = per.Enum[IABNodeIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumIABSupported = per.Enum[IABSupported]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumLTEMIndication = per.Enum[LTEMIndication]{
	Root:       []string{"lte-m"},
	Extensible: true,
}

var enumMMERelaySupportIndicator = per.Enum[MMERelaySupportIndicator]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumManagementBasedMDTAllowed = per.Enum[ManagementBasedMDTAllowed]{
	Root:       []string{"allowed"},
	Extensible: true,
}

var enumNBIoTDefaultPagingDRX = per.Enum[NBIoTDefaultPagingDRX]{
	Root:       []string{"v128", "v256", "v512", "v1024"},
	Extensible: true,
}

var enumNBIoTPagingDRX = per.Enum[NBIoTPagingDRX]{
	Root:       []string{"v32", "v64", "v128", "v256", "v512", "v1024"},
	Extensible: true,
}

var enumNotifySourceeNB = per.Enum[NotifySourceeNB]{
	Root:       []string{"notifySource"},
	Extensible: true,
}

var enumPSServiceNotAvailable = per.Enum[PSServiceNotAvailable]{
	Root:       []string{"ps-service-not-available"},
	Extensible: true,
}

var enumPagingCause = per.Enum[PagingCause]{
	Root:       []string{"voice"},
	Extensible: true,
}

var enumPendingDataIndication = per.Enum[PendingDataIndication]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumRelayNodeIndicator = per.Enum[RelayNodeIndicator]{
	Root:       []string{"true"},
	Extensible: true,
}

var enumSRVCCHOIndication = per.Enum[SRVCCHOIndication]{
	Root:       []string{"pSandCS", "cSonly"},
	Extensible: true,
}

var enumSRVCCOperationNotPossible = per.Enum[SRVCCOperationNotPossible]{
	Root:       []string{"notPossible"},
	Extensible: true,
}

var enumSRVCCOperationPossible = per.Enum[SRVCCOperationPossible]{
	Root:       []string{"possible"},
	Extensible: true,
}

var enumSecondaryRATDataUsageRequest = per.Enum[SecondaryRATDataUsageRequest]{
	Root:       []string{"requested"},
	Extensible: true,
}

var enumSubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator = per.Enum[SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator]{
	Root:       []string{"periodically", "ondemand"},
	Extensible: true,
}

var enumSubscriptionBasedUEDifferentiationInfoStationaryIndication = per.Enum[SubscriptionBasedUEDifferentiationInfoStationaryIndication]{
	Root:       []string{"stationary", "mobile"},
	Extensible: true,
}

var enumSubscriptionBasedUEDifferentiationInfoTrafficProfile = per.Enum[SubscriptionBasedUEDifferentiationInfoTrafficProfile]{
	Root:       []string{"single-packet", "dual-packets", "multiple-packets"},
	Extensible: true,
}

var enumSubscriptionBasedUEDifferentiationInfoBatteryIndication = per.Enum[SubscriptionBasedUEDifferentiationInfoBatteryIndication]{
	Root:       []string{"battery-powered", "battery-powered-not-rechargeable-or-replaceable", "not-battery-powered"},
	Extensible: true,
}

var enumUECapabilityInfoRequest = per.Enum[UECapabilityInfoRequest]{
	Root:       []string{"requested"},
	Extensible: true,
}

var enumUEUserPlaneCIoTSupportIndicator = per.Enum[UEUserPlaneCIoTSupportIndicator]{
	Root:       []string{"supported"},
	Extensible: true,
}

var enumExpectedHOInterval = per.Enum[ExpectedHOInterval]{
	Root:       []string{"sec15", "sec30", "sec60", "sec90", "sec120", "sec180", "long-time"},
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

var enumVehicleUE = per.Enum[VehicleUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumPedestrianUE = per.Enum[PedestrianUE]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumPagingeDRXCycle = per.Enum[PagingeDRXCycle]{
	Root:       []string{"hfhalf", "hf1", "hf2", "hf4", "hf6", "hf8", "hf10", "hf12", "hf14", "hf16", "hf32", "hf64", "hf128", "hf256"},
	Extensible: true,
}

var enumPagingTimeWindow = per.Enum[PagingTimeWindow]{
	Root:       []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12", "s13", "s14", "s15", "s16"},
	Extensible: true,
}

var enumProSeDirectDiscovery = per.Enum[ProSeDirectDiscovery]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumProSeDirectCommunication = per.Enum[ProSeDirectCommunication]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumProSeUEtoNetworkRelaying = per.Enum[ProSeUEtoNetworkRelaying]{
	Root:       []string{"authorized", "not-authorized"},
	Extensible: true,
}

var enumPagingProbabilityInformation = per.Enum[PagingProbabilityInformation]{
	Root:       []string{"p00", "p05", "p10", "p15", "p20", "p25", "p30", "p35", "p40", "p45", "p50", "p55", "p60", "p65", "p70", "p75", "p80", "p85", "p90", "p95", "p100"},
	Extensible: true,
}

var enumNextPagingAreaScope = per.Enum[NextPagingAreaScope]{
	Root:       []string{"same", "changed"},
	Extensible: true,
}

var enumSourceOfUEActivityBehaviourInformation = per.Enum[SourceOfUEActivityBehaviourInformation]{
	Root:       []string{"subscription-information", "statistics"},
	Extensible: true,
}

var enumSecondaryRATType = per.Enum[SecondaryRATType]{
	Root:      []string{"nR"},
	Extension: []string{"unlicensed"},
}

var enumRangeType = per.Enum[RangeType]{
	Root:       []string{"m50", "m80", "m180", "m200", "m350", "m400", "m500", "m700", "m1000"},
	Extensible: true,
}
