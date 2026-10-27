// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

var mandatoryBit = map[uint32]bool{
	tgpp.AVP3GPPChargingCharacteristics:      false,
	tgpp.AVPMaxRequestedBandwidthDL:          true,
	tgpp.AVPMaxRequestedBandwidthUL:          true,
	tgpp.AVPExtendedMaxRequestedBWDL:         false,
	tgpp.AVPExtendedMaxRequestedBWUL:         false,
	tgpp.AVPVisitedNetworkIdentifier:         false,
	tgpp.AVPConfidentialityKey:               true,
	tgpp.AVPIntegrityKey:                     true,
	tgpp.AVPMSISDN:                           true,
	AVPServedPartyIPAddress:                  true,
	tgpp.AVPQoSClassIdentifier:               true,
	tgpp.AVPRATType:                          true,
	tgpp.AVPAllocationRetentionPriority:      false,
	AVPSubscriptionData:                      true,
	AVPTerminalInformation:                   true,
	AVPIMEI:                                  true,
	AVPSoftwareVersion:                       true,
	AVPULRFlags:                              true,
	AVPULAFlags:                              true,
	AVPVisitedPLMNID:                         true,
	AVPRequestedEUTRANAuthenticationInfo:     true,
	AVPRequestedUTRANGERANAuthenticationInfo: true,
	AVPNumberOfRequestedVectors:              true,
	AVPReSynchronizationInfo:                 true,
	AVPImmediateResponsePreferred:            true,
	AVPAuthenticationInfo:                    true,
	AVPEUTRANVector:                          true,
	AVPUTRANVector:                           true,
	AVPGERANVector:                           true,
	AVPNetworkAccessMode:                     true,
	AVPHPLMNODB:                              true,
	AVPItemNumber:                            true,
	AVPCancellationType:                      true,
	AVPDSRFlags:                              true,
	AVPDSAFlags:                              true,
	AVPContextIdentifier:                     true,
	AVPSubscriberStatus:                      true,
	AVPOperatorDeterminedBarring:             true,
	AVPAccessRestrictionData:                 true,
	AVPAPNOIReplacement:                      true,
	AVPAllAPNConfigurationsIncludedIndicator: true,
	AVPAPNConfigurationProfile:               true,
	AVPAPNConfiguration:                      true,
	AVPEPSSubscribedQoSProfile:               true,
	AVPVPLMNDynamicAddressAllowed:            true,
	AVPSTNSR:                                 true,
	AVPAlertReason:                           true,
	AVPAMBR:                                  true,
	AVPPDNGWAllocationType:                   true,
	AVPRATFrequencySelectionPriorityID:       true,
	AVPIDAFlags:                              true,
	AVPPUAFlags:                              true,
	AVPNORFlags:                              true,
	AVPUserID:                                false,
	AVPRegionalSubscriptionZoneCode:          true,
	AVPRAND:                                  true,
	AVPXRES:                                  true,
	AVPAUTN:                                  true,
	AVPKASME:                                 true,
	AVPKc:                                    true,
	AVPSRES:                                  true,
	AVPPDNType:                               true,
	AVPRoamingRestrictedDueToUnsupportedFeature: true,
	AVPTraceReference:                  true,
	AVP3GPP2MEID:                       true,
	AVPSpecificAPNInfo:                 true,
	AVPSSCode:                          true,
	AVPTSCode:                          true,
	tgpp.AVPSGSNNumber:                 true,
	AVPIDRFlags:                        true,
	AVPIMSVoiceOverPSSessionsSupported: false,
	AVPHomogeneousSupportOfIMSVoiceOverPSSession: false,
	AVPLastUEActivityTime:                        false,
	AVPErrorDiagnostic:                           false,
	AVPUESRVCCCapability:                         false,
	AVPSubscribedPeriodicRAUTAUTimer:             false,
	AVPPURFlags:                                  false,
	AVPSubscribedVSRVCC:                          false,
	AVPEquivalentPLMNList:                        false,
	AVPCLRFlags:                                  false,
	AVPTimeZone:                                  false,
	AVPAMSISDN:                                   false,
	tgpp.AVPMMENumberForMTSMS:                    false,
	AVPSMSRegisterRequest:                        false,
	AVPLocalTimeZone:                             false,
	AVPDaylightSavingTime:                        false,
	AVPSGsMMEIdentity:                            false,
	AVPCoupledNodeDiameterID:                     false,
	AVPResetID:                                   false,
	AVPAIRFlags:                                  false,
	AVPUEUsageType:                               false,
	AVPNonIPPDNTypeIndicator:                     false,
	AVPNonIPDataDeliveryMechanism:                false,
	AVPAdditionalContextIdentifier:               false,
	AVPSubscriptionDataDeletion:                  false,
	AVPInterworking5GSIndicator:                  false,
	AVPEthernetPDNTypeIndicator:                  false,
	AVPThirdContextIdentifier:                    false,
	AVPSCEFID:                                    false,
	tgpp.AVPMaximumUEAvailabilityTime:            false,
}

func avpFlags(code uint32) uint8 {
	if mandatoryBit[code] {
		return diameter.AVPFlagMandatory
	}

	return 0
}

func unsignedAVP(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, avpFlags(code), tgpp.VendorID, v)
}

func octetsAVP(code uint32, v []byte) diameter.AVP {
	return diameter.OctetString(code, avpFlags(code), tgpp.VendorID, v)
}

func stringAVP(code uint32, v string) diameter.AVP {
	return diameter.UTF8String(code, avpFlags(code), tgpp.VendorID, v)
}

func groupedAVP(code uint32, avps ...diameter.AVP) diameter.AVP {
	return diameter.Grouped(code, avpFlags(code), tgpp.VendorID, avps...)
}

func ietfString(code uint32, v string) diameter.AVP {
	return diameter.UTF8String(code, diameter.AVPFlagMandatory, 0, v)
}
