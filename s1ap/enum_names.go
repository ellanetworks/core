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
