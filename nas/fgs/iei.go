// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// 5GMM information element identifiers (TS 24.501 §8.2 message definitions).
const (
	ieiAUTN                          uint8 = 0x20 // authentication parameter AUTN
	ieiRAND                          uint8 = 0x21 // authentication parameter RAND
	ieiAuthResponseParam             uint8 = 0x2D // authentication response parameter (RES*)
	ieiAuthFailureParam              uint8 = 0x30 // authentication failure parameter (AUTS)
	ieiT3502Value                    uint8 = 0x16 // T3502 value (GPRS timer 2)
	ieiT3346Value                    uint8 = 0x5F // T3346 value (GPRS timer 2)
	ieiLocalTimeZone                 uint8 = 0x46 // CONFIGURATION UPDATE COMMAND: local time zone
	ieiUniversalTimeAndLocalTimeZone uint8 = 0x47 // CONFIGURATION UPDATE COMMAND
	ieiNetworkDaylightSavingTime     uint8 = 0x49 // CONFIGURATION UPDATE COMMAND
	ieiSORContainer                  uint8 = 0x73 // SOR transparent container
	ieiEAPMessage                    uint8 = 0x78
	ieiPduSessionID2                 uint8 = 0x12 // PDU session identity 2
	ieiOldPDUSessionID               uint8 = 0x59 // old PDU session identity 2
	ieiAdditionalInfo                uint8 = 0x24 // additional information
	ieiBackoffTimer                  uint8 = 0x37 // DL NAS TRANSPORT / PDU SESSION MODIFICATION REJECT: back-off timer value
	ieiCongestionReattempt           uint8 = 0x61 // 5GSM congestion re-attempt indicator
	ieiReattemptIndicator            uint8 = 0x1D // re-attempt indicator
	ieiCause5GMM                     uint8 = 0x58 // 5GMM cause
	ieiIMEISVRequest                 uint8 = 0xE0 // IMEISV request (type 1)
	ieiAdditional5GSec               uint8 = 0x36 // additional 5G security information
	ieiPDUSessionStatus              uint8 = 0x50 // PDU session status
	ieiPDUReactResult                uint8 = 0x26 // PDU session reactivation result
	ieiPDUReactErrCause              uint8 = 0x72 // PDU session reactivation result error cause
	ieiGUTI5G                        uint8 = 0x77 // 5G-GUTI (5GS mobile identity)
	ieiEquivalentPlmns               uint8 = 0x4A // equivalent PLMNs
	ieiTAIList                       uint8 = 0x54 // TAI list
	ieiAllowedNSSAI                  uint8 = 0x15 // allowed NSSAI
	ieiNetworkFeature                uint8 = 0x21 // 5GS network feature support
	ieiT3512Value                    uint8 = 0x5E // T3512 value (GPRS timer 3)
	ieiNegotiatedDRX                 uint8 = 0x51 // negotiated DRX parameters
	ieiConfigUpdateInd               uint8 = 0xD0 // configuration update indication (type 1)
	ieiFullNameForNet                uint8 = 0x43 // full name for network
	ieiShortNameForNet               uint8 = 0x45 // short name for network
)

// Per-message 5GMM IEIs. An IEI's meaning is message-scoped (TS 24.007 §11.2.4),
// so the same octet can name different IEs in different messages (e.g. 0x77 is the
// additional GUTI in REGISTRATION REQUEST but the IMEISV in SECURITY MODE COMPLETE);
// each is named for its meaning, sharing the value where 3GPP reuses it.
const (
	ieiGMMCapability           uint8 = 0x10 // REGISTRATION REQUEST: 5GMM capability
	ieiS1UENetworkCapability   uint8 = 0x17 // REGISTRATION REQUEST: S1 UE network capability
	ieiUEUsageSetting          uint8 = 0x18 // REGISTRATION REQUEST: UE's usage setting
	ieiAllowedPDUSessionStatus uint8 = 0x25 // allowed PDU session status
	ieiReplayedS1UESecCap      uint8 = 0x19 // SECURITY MODE COMMAND: replayed S1 UE security capabilities
	ieiUEStatus                uint8 = 0x2B // REGISTRATION REQUEST: UE status
	ieiABBA                    uint8 = 0x38 // SECURITY MODE COMMAND: ABBA
	ieiSelectedEPSNASSecAlg    uint8 = 0x57 // SECURITY MODE COMMAND: selected EPS NAS security algorithms
	ieiUESecurityCapability    uint8 = 0x2E // REGISTRATION REQUEST: UE security capability
	ieiRequestedNSSAI          uint8 = 0x2F // REGISTRATION REQUEST: requested NSSAI
	ieiUplinkDataStatus        uint8 = 0x40 // uplink data status
	ieiRequestedDRXParameters  uint8 = 0x51 // requested DRX parameters
	ieiLastVisitedTAI          uint8 = 0x52 // REGISTRATION REQUEST: last visited registered TAI
	ieiUpdateType5GS           uint8 = 0x53 // REGISTRATION REQUEST: 5GS update type
	ieiEPSBearerContextStatus  uint8 = 0x60 // REGISTRATION REQUEST: EPS bearer context status
	ieiEPSNASMessageContainer  uint8 = 0x70 // REGISTRATION REQUEST: EPS NAS message container
	ieiNASMessageContainer     uint8 = 0x71 // NAS message container
	ieiLADNIndication          uint8 = 0x74 // REGISTRATION REQUEST: LADN indication
	ieiPayloadContainer        uint8 = 0x7B // REGISTRATION REQUEST: payload container
	ieiRequestType             uint8 = 0x80 // UL NAS TRANSPORT: request type (type 1, walker-emitted IEI)
	ieiMICOIndication          uint8 = 0xB0 // REGISTRATION REQUEST: MICO indication (type 1)
	ieiIMEISV                  uint8 = 0x77 // SECURITY MODE COMPLETE: IMEISV (same octet as the additional GUTI)

	// REGISTRATION ACCEPT optional IEs (TS 24.501 §8.2.7).
	ieiRejectedNSSAI          uint8 = 0x11 // rejected NSSAI
	ieiConfiguredNSSAI        uint8 = 0x31 // configured NSSAI
	ieiServiceAreaList        uint8 = 0x27 // service area list
	ieiEmergencyNumberList    uint8 = 0x34 // emergency number list
	ieiNon3GppDeregTimer      uint8 = 0x5D // non-3GPP de-registration timer
	ieiOperatorAccessCategory uint8 = 0x76 // operator-defined access category definitions
	ieiExtendedLADNInfo       uint8 = 0x01 // REGISTRATION ACCEPT: extended LADN information
	ieiSNSSAILocationValidity uint8 = 0x02 // REGISTRATION ACCEPT: S-NSSAI location validity
	ieiPartiallyAllowedNSSAI  uint8 = 0x03 // REGISTRATION ACCEPT: partially allowed NSSAI
	ieiPartiallyRejectedNSSAI uint8 = 0x04 // REGISTRATION ACCEPT: partially rejected NSSAI
	ieiLADNInformation        uint8 = 0x79 // LADN information
	ieiExtEmergencyNumberList uint8 = 0x7A // extended emergency number list
	ieiNetworkSlicingInd      uint8 = 0x90 // network slicing indication (type 1)
	ieiNSSAIInclusionMode     uint8 = 0xA0 // NSSAI inclusion mode (type 1)
	ieiNon3GppNwPolicies      uint8 = 0xD0 // non-3GPP NW policies (type 1)
)
