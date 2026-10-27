// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import "github.com/ellanetworks/core/diameter/tgpp"

type CancellationType uint32

const (
	CancellationMMEUpdateProcedure          CancellationType = 0
	CancellationSGSNUpdateProcedure         CancellationType = 1
	CancellationSubscriptionWithdrawal      CancellationType = 2
	CancellationUpdateProcedureIWF          CancellationType = 3
	CancellationInitialAttachProcedure      CancellationType = 4
	CancellationDisasterConditionTerminated CancellationType = 5
)

var cancellationTypeNames = tgpp.EnumNames{
	"MME_UPDATE_PROCEDURE", "SGSN_UPDATE_PROCEDURE", "SUBSCRIPTION_WITHDRAWAL", "UPDATE_PROCEDURE_IWF",
	"INITIAL_ATTACH_PROCEDURE", "DISASTER_CONDITION_TERMINATED",
}

func (t CancellationType) String() string {
	return cancellationTypeNames.Name("CancellationType", uint32(t))
}

func (t CancellationType) valid() bool { return cancellationTypeNames.Has(uint32(t)) }

type SubscriberStatus uint32

const (
	SubscriberStatusServiceGranted            SubscriberStatus = 0
	SubscriberStatusOperatorDeterminedBarring SubscriberStatus = 1
)

var subscriberStatusNames = tgpp.EnumNames{"SERVICE_GRANTED", "OPERATOR_DETERMINED_BARRING"}

func (s SubscriberStatus) String() string {
	return subscriberStatusNames.Name("SubscriberStatus", uint32(s))
}

func (s SubscriberStatus) valid() bool { return subscriberStatusNames.Has(uint32(s)) }

type NetworkAccessMode uint32

const (
	NetworkAccessPacketAndCircuit NetworkAccessMode = 0
	NetworkAccessOnlyPacket       NetworkAccessMode = 2
)

var networkAccessModeNames = tgpp.EnumNames{"PACKET_AND_CIRCUIT", "", "ONLY_PACKET"}

func (m NetworkAccessMode) String() string {
	return networkAccessModeNames.Name("NetworkAccessMode", uint32(m))
}

func (m NetworkAccessMode) valid() bool { return networkAccessModeNames.Has(uint32(m)) }

type PDNType uint32

const (
	PDNTypeIPv4       PDNType = 0
	PDNTypeIPv6       PDNType = 1
	PDNTypeIPv4v6     PDNType = 2
	PDNTypeIPv4OrIPv6 PDNType = 3
)

var pdnTypeNames = tgpp.EnumNames{"IPv4", "IPv6", "IPv4v6", "IPv4_OR_IPv6"}

func (t PDNType) String() string { return pdnTypeNames.Name("PDNType", uint32(t)) }

func (t PDNType) valid() bool { return pdnTypeNames.Has(uint32(t)) }

type PDNGWAllocationType uint32

const (
	PDNGWAllocationStatic  PDNGWAllocationType = 0
	PDNGWAllocationDynamic PDNGWAllocationType = 1
)

var pdnGWAllocationTypeNames = tgpp.EnumNames{"STATIC", "DYNAMIC"}

func (t PDNGWAllocationType) String() string {
	return pdnGWAllocationTypeNames.Name("PDNGWAllocationType", uint32(t))
}

func (t PDNGWAllocationType) valid() bool { return pdnGWAllocationTypeNames.Has(uint32(t)) }

type APNConfigurationsIncluded uint32

const (
	AllAPNConfigurationsIncluded           APNConfigurationsIncluded = 0
	ModifiedAddedAPNConfigurationsIncluded APNConfigurationsIncluded = 1
)

var apnConfigurationsIncludedNames = tgpp.EnumNames{
	"All_APN_CONFIGURATIONS_INCLUDED", "MODIFIED_ADDED_APN_CONFIGURATIONS_INCLUDED",
}

func (i APNConfigurationsIncluded) String() string {
	return apnConfigurationsIncludedNames.Name("APNConfigurationsIncluded", uint32(i))
}

func (i APNConfigurationsIncluded) valid() bool { return apnConfigurationsIncludedNames.Has(uint32(i)) }

type ErrorDiagnostic uint32

const (
	ErrorDiagnosticGPRSDataSubscribed   ErrorDiagnostic = 0
	ErrorDiagnosticNoGPRSDataSubscribed ErrorDiagnostic = 1
	ErrorDiagnosticODBAllAPN            ErrorDiagnostic = 2
	ErrorDiagnosticODBHPLMNAPN          ErrorDiagnostic = 3
	ErrorDiagnosticODBVPLMNAPN          ErrorDiagnostic = 4
)

var errorDiagnosticNames = tgpp.EnumNames{
	"GPRS_DATA_SUBSCRIBED", "NO_GPRS_DATA_SUBSCRIBED", "ODB_ALL_APN", "ODB_HPLMN_APN", "ODB_VPLMN_APN",
}

func (d ErrorDiagnostic) String() string {
	return errorDiagnosticNames.Name("ErrorDiagnostic", uint32(d))
}

type IMSVoiceOverPS uint32

const (
	IMSVoiceOverPSNotSupported IMSVoiceOverPS = 0
	IMSVoiceOverPSSupported    IMSVoiceOverPS = 1
)

var imsVoiceOverPSNames = tgpp.EnumNames{"NOT_SUPPORTED", "SUPPORTED"}

func (s IMSVoiceOverPS) String() string { return imsVoiceOverPSNames.Name("IMSVoiceOverPS", uint32(s)) }

func (s IMSVoiceOverPS) valid() bool { return imsVoiceOverPSNames.Has(uint32(s)) }

type UESRVCCCapability uint32

const (
	UESRVCCNotSupported UESRVCCCapability = 0
	UESRVCCSupported    UESRVCCCapability = 1
)

var ueSRVCCCapabilityNames = tgpp.EnumNames{"UE-SRVCC-NOT-SUPPORTED", "UE-SRVCC-SUPPORTED"}

func (c UESRVCCCapability) String() string {
	return ueSRVCCCapabilityNames.Name("UESRVCCCapability", uint32(c))
}

func (c UESRVCCCapability) valid() bool { return ueSRVCCCapabilityNames.Has(uint32(c)) }

type AlertReason uint32

const (
	AlertReasonUEPresent         AlertReason = 0
	AlertReasonUEMemoryAvailable AlertReason = 1
)

var alertReasonNames = tgpp.EnumNames{"UE_PRESENT", "UE_MEMORY_AVAILABLE"}

func (r AlertReason) String() string { return alertReasonNames.Name("AlertReason", uint32(r)) }

func (r AlertReason) valid() bool { return alertReasonNames.Has(uint32(r)) }

type SMSRegisterRequest uint32

const (
	SMSRegistrationRequired     SMSRegisterRequest = 0
	SMSRegistrationNotPreferred SMSRegisterRequest = 1
	SMSRegistrationNoPreference SMSRegisterRequest = 2
)

var smsRegisterRequestNames = tgpp.EnumNames{"SMS_REGISTRATION_REQUIRED", "SMS_REGISTRATION_NOT_PREFERRED", "NO_PREFERENCE"}

func (r SMSRegisterRequest) String() string {
	return smsRegisterRequestNames.Name("SMSRegisterRequest", uint32(r))
}

func (r SMSRegisterRequest) valid() bool { return smsRegisterRequestNames.Has(uint32(r)) }

type DaylightSavingTime uint32

const (
	DaylightSavingNoAdjustment DaylightSavingTime = 0
	DaylightSavingPlusOneHour  DaylightSavingTime = 1
	DaylightSavingPlusTwoHours DaylightSavingTime = 2
)

var daylightSavingTimeNames = tgpp.EnumNames{"NO_ADJUSTMENT", "PLUS_ONE_HOUR_ADJUSTMENT", "PLUS_TWO_HOURS_ADJUSTMENT"}

func (d DaylightSavingTime) String() string {
	return daylightSavingTimeNames.Name("DaylightSavingTime", uint32(d))
}

func (d DaylightSavingTime) valid() bool { return daylightSavingTimeNames.Has(uint32(d)) }

const (
	enumFalse uint32 = 0
	enumTrue  uint32 = 1
)

const (
	vplmnDynamicAddressNotAllowed uint32 = 0
	vplmnDynamicAddressAllowed    uint32 = 1
	roamingRestrictedValue        uint32 = 0
	vsrvccSubscribedValue         uint32 = 0
	preemptionEnabled             uint32 = 0
	preemptionDisabled            uint32 = 1
)
