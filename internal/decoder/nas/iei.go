// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

// 5GMM information element identifiers this decoder renders (TS 24.501 §8.2). An
// IEI's meaning is message-scoped (TS 24.007 §11.2.4); each is named for its meaning.
const (
	ieiCapability5GMM          uint8 = 0x10 // 5GMM capability
	ieiT3502Value              uint8 = 0x16 // T3502 value
	ieiS1UENetworkCapability   uint8 = 0x17 // S1 UE network capability
	ieiUesUsageSetting         uint8 = 0x18 // UE's usage setting
	ieiReplayedS1UESecCap      uint8 = 0x19 // replayed S1 UE security capabilities
	ieiAUTN                    uint8 = 0x20 // authentication parameter AUTN
	ieiRAND                    uint8 = 0x21 // authentication parameter RAND
	ieiAllowedPDUSessionStat   uint8 = 0x25 // allowed PDU session status
	ieiPDUReactResult          uint8 = 0x26 // PDU session reactivation result
	ieiCapability5GSM          uint8 = 0x28 // 5GSM capability
	ieiUEStatus                uint8 = 0x2b // UE status
	ieiAuthResponseParam       uint8 = 0x2d // authentication response parameter (RES*)
	ieiUESecurityCapability    uint8 = 0x2e // UE security capability
	ieiRequestedNSSAI          uint8 = 0x2f // requested NSSAI
	ieiAuthFailureParam        uint8 = 0x30 // authentication failure parameter (AUTS)
	ieiAdditional5GSecInfo     uint8 = 0x36 // additional 5G security information
	ieiABBA                    uint8 = 0x38 // ABBA
	ieiSMPDUDNRequest          uint8 = 0x39 // SM PDU DN request container
	ieiUplinkDataStatus        uint8 = 0x40 // uplink data status
	ieiMaxPacketFilters        uint8 = 0x55 // maximum number of supported packet filters
	ieiPDUSessionStatus        uint8 = 0x50 // PDU session status
	ieiSelectedEPSNASSecAlg    uint8 = 0x57 // selected EPS NAS security algorithms
	ieiRequestedDRXParameters  uint8 = 0x51 // requested DRX parameters
	ieiLastVisitedTAI          uint8 = 0x52 // last visited registered TAI
	ieiUpdateType5GS           uint8 = 0x53 // 5GS update type
	ieiT3346Value              uint8 = 0x5f // T3346 value
	ieiEPSBearerContextStatus  uint8 = 0x60 // EPS bearer context status
	ieiPDUReactErrCause        uint8 = 0x72 // PDU session reactivation result error cause
	ieiMappedEPSBearerContexts uint8 = 0x75 // mapped EPS bearer contexts
	ieiEPSNASMessageContainer  uint8 = 0x70 // EPS NAS message container
	ieiNASMessageContainer     uint8 = 0x71 // NAS message container
	ieiSORContainer            uint8 = 0x73 // SOR transparent container
	ieiLADNIndication          uint8 = 0x74 // LADN indication
	ieiAdditionalGUTI          uint8 = 0x77 // additional GUTI
	ieiEAPMessage              uint8 = 0x78 // EAP message
	ieiPayloadContainer        uint8 = 0x7b // payload container

	ieiIMEISV      uint8 = 0x77 // SECURITY MODE COMPLETE IMEISV (same octet as the additional GUTI)
	ieiExtendedPCO uint8 = 0x7b // extended protocol configuration options (same octet as the payload container)

	// Type-1 IEs: the walker emits their IEI as the octet's high nibble. Some octets
	// are message-scoped (0x90 is network slicing in GMM but PDU session type in GSM).
	ieiNetworkSlicingIndication uint8 = 0x90 // REGISTRATION REQUEST network slicing indication
	ieiPDUSessionType           uint8 = 0x90 // PDU SESSION ESTABLISHMENT REQUEST PDU session type
	ieiSSCMode                  uint8 = 0xa0 // PDU SESSION ESTABLISHMENT REQUEST SSC mode
	ieiMICOIndication           uint8 = 0xb0 // REGISTRATION REQUEST MICO indication
	ieiAlwaysonPDUSession       uint8 = 0xb0 // PDU SESSION ESTABLISHMENT REQUEST always-on PDU session requested
	ieiNoncurrentNativeNASKSI   uint8 = 0xc0 // non-current native NAS key set identifier
	ieiIMEISVRequest            uint8 = 0xe0 // IMEISV request
)
