// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

// EMM information element identifiers (TS 24.301 §8.2 message definitions). An
// optional IE's IEI is message-scoped, so the same octet can name different IEs in
// different messages (e.g. 0x50 is the assigned GUTI downlink but the additional
// GUTI uplink); each is named for its meaning, sharing the value where reused.
const (
	ieiIMEISVRequest                  uint8 = 0xC0 // SECURITY MODE COMMAND: IMEISV request (type 1)
	ieiTMSIBasedNRIContainer          uint8 = 0x10
	ieiMobileStationClassmark2        uint8 = 0x11
	ieiOldLocationAreaID              uint8 = 0x13
	ieiLocationAreaID                 uint8 = 0x13 // ATTACH ACCEPT / TAU ACCEPT (same octet as the old LAI uplink)
	ieiT3402Value                     uint8 = 0x16 // ATTACH REJECT T3402 (TLV); distinct from the ATTACH ACCEPT T3402 (0x17, TV)
	ieiAdditionalInformationRequested uint8 = 0x17
	ieiT3402ValueAccept               uint8 = 0x17 // ATTACH ACCEPT / TAU ACCEPT T3402 (TV); the ATTACH REJECT one is 0x16 (TLV)
	ieiOldPTMSISignature              uint8 = 0x19
	ieiMobileStationClassmark3        uint8 = 0x20
	ieiIMEISV                         uint8 = 0x23 // SECURITY MODE COMPLETE: IMEISV mobile identity
	ieiMSNetworkCapability            uint8 = 0x31
	ieiN1UENetworkCapability          uint8 = 0x32
	ieiNegotiatedLLCSAPI              uint8 = 0x32 // ESM bearer context messages (same octet as the N1 UE network capability in EMM)
	ieiUERadioCapabilityIDAvail       uint8 = 0x34
	ieiRequestedWUSAssistance         uint8 = 0x35
	ieiDRXParameterNBS1Mode           uint8 = 0x36
	ieiRequestedIMSIOffset            uint8 = 0x38
	ieiSupportedCodecs                uint8 = 0x40
	ieiFullNameForNetwork             uint8 = 0x43 // EMM INFORMATION
	ieiShortNameForNetwork            uint8 = 0x45 // EMM INFORMATION
	ieiLocalTimeZone                  uint8 = 0x46 // EMM INFORMATION
	ieiUniversalTimeAndLocalTimeZone  uint8 = 0x47 // EMM INFORMATION
	ieiNetworkDaylightSavingTime      uint8 = 0x49 // EMM INFORMATION
	ieiHashMME                        uint8 = 0x4F // SECURITY MODE COMMAND
	ieiReplayedNASMessage             uint8 = 0x79 // SECURITY MODE COMPLETE (TS 24.301 table 8.2.21.1)
	ieiGUTI                           uint8 = 0x50 // ATTACH ACCEPT / TAU ACCEPT: assigned GUTI
	ieiAdditionalGUTI                 uint8 = 0x50 // ATTACH REQUEST / TAU REQUEST (same octet as the assigned GUTI)
	ieiLastVisitedRegisteredTAI       uint8 = 0x52
	ieiEMMCause                       uint8 = 0x53 // ATTACH ACCEPT / TAU ACCEPT / network DETACH REQUEST
	ieiT3442Value                     uint8 = 0x5B // SERVICE REJECT (GPRS timer)
	ieiT3346Value                     uint8 = 0x5F // SERVICE REJECT (GPRS timer 2)
	ieiT3448Value                     uint8 = 0x6B // SERVICE REJECT (GPRS timer 2)
	ieiLowerBoundTimer                uint8 = 0x1C // SERVICE REJECT (GPRS timer 3)
	ieiForbiddenTAIRoaming            uint8 = 0x1D // SERVICE REJECT: forbidden TAIs for roaming
	ieiForbiddenTAIRegional           uint8 = 0x1E // SERVICE REJECT: forbidden TAIs for regional provision of service
	ieiNonceUE                        uint8 = 0x55 // TAU REQUEST
	ieiReplayedNonceUE                uint8 = 0x55 // SECURITY MODE COMMAND
	ieiNonceMME                       uint8 = 0x56 // SECURITY MODE COMMAND
	ieiEPSBearerContextStatus         uint8 = 0x57 // TAU REQUEST / ACCEPT
	ieiUENetworkCapability            uint8 = 0x58 // TAU REQUEST (same octet as the ESM cause in ESM messages)
	ieiDRXParameter                   uint8 = 0x5C
	ieiVoiceDomainPreference          uint8 = 0x5D
	ieiT3412ExtendedValue             uint8 = 0x5E
	ieiT3412Value                     uint8 = 0x5A // TAU ACCEPT (GPRS timer)
	ieiT3423Value                     uint8 = 0x59 // ATTACH ACCEPT / TAU ACCEPT (GPRS timer)
	ieiNetworkFeatureSupport          uint8 = 0x64 // ATTACH ACCEPT / TAU ACCEPT
	ieiT3324Value                     uint8 = 0x6A
	ieiUEStatus                       uint8 = 0x6D
	ieiExtendedDRXParameters          uint8 = 0x6E
	ieiUEAdditionalSecurityCap        uint8 = 0x6F

	// Type-1 (TV, ½ octet) IEs: the IEI is the high nibble, the value the low nibble.
	ieiTMSIStatus              uint8 = 0x90
	ieiMSNetworkFeatureSupport uint8 = 0xC0
	ieiDeviceProperties        uint8 = 0xD0
	ieiOldGUTIType             uint8 = 0xE0
	ieiAdditionalUpdateType    uint8 = 0xF0
)
