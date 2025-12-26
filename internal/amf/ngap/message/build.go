// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func BuildPDUSessionResourceReleaseCommand(amfUENgapID int64, ranUENgapID int64, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceRelease
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand
	initiatingMessage.Value.PDUSessionResourceReleaseCommand = new(ngapType.PDUSessionResourceReleaseCommand)

	pDUSessionResourceReleaseCommand := initiatingMessage.Value.PDUSessionResourceReleaseCommand
	PDUSessionResourceReleaseCommandIEs := &pDUSessionResourceReleaseCommand.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENgapID

	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENgapID

	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	// NAS-PDU (optional)
	if nasPdu != nil {
		ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASPDU
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentNASPDU
		ie.Value.NASPDU = new(ngapType.NASPDU)

		ie.Value.NASPDU.Value = nasPdu

		PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)
	}

	// PDUSessionResourceToReleaseListRelCmd
	ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentPDUSessionResourceToReleaseListRelCmd
	ie.Value.PDUSessionResourceToReleaseListRelCmd = &pduSessionResourceReleasedList
	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	return ngap.Encoder(pdu)
}

/*The PGW-C+SMF (V-SMF in the case of home-routed roaming scenario only) sends
a Nsmf_PDUSession_CreateSMContext Response(N2 SM Information (PDU Session ID, cause code)) to the AMF.*/
// Cause is from SMF
// pduSessionResourceSetupList provided by AMF, and the transfer data is from SMF
// sourceToTargetTransparentContainer is received from S-RAN
// nsci: new security context indicator, if amfUe has updated security context,
// set nsci to true, otherwise set to false
func BuildHandoverRequest(
	amfUeNgapID int64,
	targetUEHandoverType ngapType.HandoverType,
	ambrUplink string,
	ambrDownlink string,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	supportedPLMN *models.PlmnSupportItem,
	supportedGUAMI *models.Guami,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeHandoverResourceAllocation
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentHandoverRequest
	initiatingMessage.Value.HandoverRequest = new(ngapType.HandoverRequest)

	handoverRequest := initiatingMessage.Value.HandoverRequest
	handoverRequestIEs := &handoverRequest.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// Handover Type
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)

	handoverType := ie.Value.HandoverType
	handoverType.Value = targetUEHandoverType.Value

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// Cause
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestIEsPresentCause
	ie.Value.Cause = &cause

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// UE Aggregate Maximum Bit Rate
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUEAggregateMaximumBitRate
	ie.Value.UEAggregateMaximumBitRate = new(ngapType.UEAggregateMaximumBitRate)

	ueAmbrUL := ngapConvert.UEAmbrToInt64(ambrUplink)
	ueAmbrDL := ngapConvert.UEAmbrToInt64(ambrDownlink)
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value = ueAmbrUL
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value = ueAmbrDL

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// UE Security Capabilities
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)

	ueSecurityCapabilities := ie.Value.UESecurityCapabilities

	nrEncryptionAlgorighm := []byte{0x00, 0x00}
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA1_128_5G() << 7
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA2_128_5G() << 6
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA3_128_5G() << 5
	ueSecurityCapabilities.NRencryptionAlgorithms.Value = ngapConvert.ByteToBitString(nrEncryptionAlgorighm, 16)

	nrIntegrityAlgorithm := []byte{0x00, 0x00}
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA1_128_5G() << 7
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA2_128_5G() << 6
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA3_128_5G() << 5
	ueSecurityCapabilities.NRintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(nrIntegrityAlgorithm, 16)

	// only support NR algorithms
	eutraEncryptionAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAencryptionAlgorithms.Value = ngapConvert.ByteToBitString(eutraEncryptionAlgorithm, 16)

	eutraIntegrityAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(eutraIntegrityAlgorithm, 16)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// Security Context
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityContext
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentSecurityContext
	ie.Value.SecurityContext = new(ngapType.SecurityContext)

	securityContext := ie.Value.SecurityContext
	securityContext.NextHopChainingCount.Value = int64(ncc)
	securityContext.NextHopNH.Value = ngapConvert.HexToBitString(hex.EncodeToString(nh), 256)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// PDU Session Resource Setup List
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentPDUSessionResourceSetupListHOReq
	ie.Value.PDUSessionResourceSetupListHOReq = &pduSessionResourceSetupListHOReq
	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// Allowed NSSAI
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAllowedNSSAI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentAllowedNSSAI
	ie.Value.AllowedNSSAI = new(ngapType.AllowedNSSAI)

	allowedNSSAI := ie.Value.AllowedNSSAI

	ngapSnssai, err := util.SNssaiToNgap(supportedPLMN.SNssai)
	if err != nil {
		return nil, fmt.Errorf("error converting snssai to ngap: %s", err)
	}

	allowedNSSAIItem := ngapType.AllowedNSSAIItem{
		SNSSAI: ngapSnssai,
	}

	allowedNSSAI.List = append(allowedNSSAI.List, allowedNSSAIItem)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	// Source To Target Transparent Container
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSourceToTargetTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentSourceToTargetTransparentContainer
	ie.Value.SourceToTargetTransparentContainer = new(ngapType.SourceToTargetTransparentContainer)

	sourceToTargetTransparentContaine := ie.Value.SourceToTargetTransparentContainer
	sourceToTargetTransparentContaine.Value = sourceToTargetTransparentContainer.Value

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)
	// GUAMI
	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGUAMI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentGUAMI
	ie.Value.GUAMI = new(ngapType.GUAMI)

	guami := ie.Value.GUAMI
	plmnID := &guami.PLMNIdentity
	amfRegionID := &guami.AMFRegionID
	amfSetID := &guami.AMFSetID
	amfPtrID := &guami.AMFPointer

	ngapPlmnID, err := util.PlmnIDToNgap(*supportedGUAMI.PlmnID)
	if err != nil {
		return nil, fmt.Errorf("error converting plmn id to ngap: %s", err)
	}
	*plmnID = *ngapPlmnID
	amfRegionID.Value, amfSetID.Value, amfPtrID.Value = ngapConvert.AmfIdToNgap(supportedGUAMI.AmfID)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	return ngap.Encoder(pdu)
}

// pduSessionResourceSwitchedList: provided by AMF, and the transfer data is from SMF
// pduSessionResourceReleasedList: provided by AMF, and the transfer data is from SMF
// newSecurityContextIndicator: if AMF has activated a new 5G NAS security context,
// set it to true, otherwise set to false
// coreNetworkAssistanceInformation: provided by AMF,
// based on collection of UE behaviour statistics and/or other available
// information about the expected UE behaviour. TS 23.501 5.4.6, 5.4.6.2
// rrcInactiveTransitionReportRequest: configured by amf
// criticalityDiagnostics: from received node when received not comprehended IE or missing IE
func BuildPathSwitchRequestAcknowledge(
	amfUeNgapID int64,
	ranUeNgapID int64,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList,
	pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck,
	newSecurityContextIndicator bool,
	coreNetworkAssistanceInformation *ngapType.CoreNetworkAssistanceInformation,
	rrcInactiveTransitionReportRequest *ngapType.RRCInactiveTransitionReportRequest,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
	supportedPLMN *models.PlmnSupportItem,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge
	successfulOutcome.Value.PathSwitchRequestAcknowledge = new(ngapType.PathSwitchRequestAcknowledge)

	pathSwitchRequestAck := successfulOutcome.Value.PathSwitchRequestAcknowledge
	pathSwitchRequestAckIEs := &pathSwitchRequestAck.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// UE Security Capabilities (optional)
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)

	ueSecurityCapabilities := ie.Value.UESecurityCapabilities
	nrEncryptionAlgorighm := []byte{0x00, 0x00}
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA1_128_5G() << 7
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA2_128_5G() << 6
	nrEncryptionAlgorighm[0] |= ueSecurityCapability.GetEA3_128_5G() << 5
	ueSecurityCapabilities.NRencryptionAlgorithms.Value = ngapConvert.ByteToBitString(nrEncryptionAlgorighm, 16)

	nrIntegrityAlgorithm := []byte{0x00, 0x00}
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA1_128_5G() << 7
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA2_128_5G() << 6
	nrIntegrityAlgorithm[0] |= ueSecurityCapability.GetIA3_128_5G() << 5
	ueSecurityCapabilities.NRintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(nrIntegrityAlgorithm, 16)

	// only support NR algorithms
	eutraEncryptionAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAencryptionAlgorithms.Value = ngapConvert.ByteToBitString(eutraEncryptionAlgorithm, 16)

	eutraIntegrityAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(eutraIntegrityAlgorithm, 16)

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// Security Context
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityContext
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentSecurityContext
	ie.Value.SecurityContext = new(ngapType.SecurityContext)

	securityContext := ie.Value.SecurityContext
	securityContext.NextHopChainingCount.Value = int64(ncc)
	securityContext.NextHopNH.Value = ngapConvert.HexToBitString(hex.EncodeToString(nh), 256)

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// New Security Context Indicator (optional)
	if newSecurityContextIndicator {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNewSecurityContextInd
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentNewSecurityContextInd
		ie.Value.NewSecurityContextInd = new(ngapType.NewSecurityContextInd)
		ie.Value.NewSecurityContextInd.Value = ngapType.NewSecurityContextIndPresentTrue
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	// PDU Session Resource Switched List
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSwitchedList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentPDUSessionResourceSwitchedList
	ie.Value.PDUSessionResourceSwitchedList = &pduSessionResourceSwitchedList
	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// PDU Session Resource Released List
	if len(pduSessionResourceReleasedList.List) > 0 {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentPDUSessionResourceReleasedListPSAck
		ie.Value.PDUSessionResourceReleasedListPSAck = &pduSessionResourceReleasedList
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	// Allowed NSSAI
	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAllowedNSSAI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentAllowedNSSAI
	ie.Value.AllowedNSSAI = new(ngapType.AllowedNSSAI)

	allowedNSSAI := ie.Value.AllowedNSSAI

	ngapSnssai, err := util.SNssaiToNgap(supportedPLMN.SNssai)
	if err != nil {
		return nil, fmt.Errorf("error converting snssai to ngap: %s", err)
	}

	allowedNSSAIItem := ngapType.AllowedNSSAIItem{
		SNSSAI: ngapSnssai,
	}

	allowedNSSAI.List = append(allowedNSSAI.List, allowedNSSAIItem)

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	// Core Network Assistance Information (optional)
	if coreNetworkAssistanceInformation != nil {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCoreNetworkAssistanceInformation
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentCoreNetworkAssistanceInformation
		ie.Value.CoreNetworkAssistanceInformation = coreNetworkAssistanceInformation
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	// RRC Inactive Transition Report Request (optional)
	if rrcInactiveTransitionReportRequest != nil {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentRRCInactiveTransitionReportRequest
		ie.Value.RRCInactiveTransitionReportRequest = rrcInactiveTransitionReportRequest
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	// Criticality Diagnostics (optional)
	if criticalityDiagnostics != nil {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

// Paging Priority: is included only if the AMF receives an Namf_Communication_N1N2MessageTransfer message
// with an ARP value associated with
// priority services (e.g., MPS, MCS), as configured by the operator. (TS 23.502 4.2.3.3, TS 23.501 5.22.3)
func BuildPaging(
	guti string,
	registrationArea []models.Tai,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueInfoOnRecommendedCellsAndRanNodesForPaging *models.InfoOnRecommendedCellsAndRanNodesForPaging,
	pagingPriority *ngapType.PagingPriority,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePaging
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPaging
	initiatingMessage.Value.Paging = new(ngapType.Paging)

	paging := initiatingMessage.Value.Paging
	pagingIEs := &paging.ProtocolIEs

	// UE Paging Identity
	ie := ngapType.PagingIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEPagingIdentity
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PagingIEsPresentUEPagingIdentity
	ie.Value.UEPagingIdentity = new(ngapType.UEPagingIdentity)

	uePagingIdentity := ie.Value.UEPagingIdentity
	uePagingIdentity.Present = ngapType.UEPagingIdentityPresentFiveGSTMSI
	uePagingIdentity.FiveGSTMSI = new(ngapType.FiveGSTMSI)

	var amfID string
	var tmsi string
	if len(guti) == 19 {
		amfID = guti[5:11]
		tmsi = guti[11:]
	} else {
		amfID = guti[6:12]
		tmsi = guti[12:]
	}
	_, amfSetID, amfPointer := ngapConvert.AmfIdToNgap(amfID)

	var err error
	uePagingIdentity.FiveGSTMSI.AMFSetID.Value = amfSetID
	uePagingIdentity.FiveGSTMSI.AMFPointer.Value = amfPointer
	uePagingIdentity.FiveGSTMSI.FiveGTMSI.Value, err = hex.DecodeString(tmsi)
	if err != nil {
		return nil, fmt.Errorf("could not decode tmsi: %s", err)
	}

	pagingIEs.List = append(pagingIEs.List, ie)

	// Paging DRX (optional)

	// TAI List for Paging
	ie = ngapType.PagingIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTAIListForPaging
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PagingIEsPresentTAIListForPaging
	ie.Value.TAIListForPaging = new(ngapType.TAIListForPaging)

	if registrationArea == nil {
		return nil, fmt.Errorf("registration area is empty for ue")
	}

	taiListForPaging := ie.Value.TAIListForPaging

	for _, tai := range registrationArea {
		var tac []byte
		taiListforPagingItem := ngapType.TAIListForPagingItem{}
		plmnID, err := util.PlmnIDToNgap(*tai.PlmnID)
		if err != nil {
			return nil, fmt.Errorf("error converting plmn id to ngap: %s", err)
		}
		taiListforPagingItem.TAI.PLMNIdentity = *plmnID
		tac, err = hex.DecodeString(tai.Tac)
		if err != nil {
			return nil, fmt.Errorf("could not decode tac: %s", err)
		}
		taiListforPagingItem.TAI.TAC.Value = tac
		taiListForPaging.List = append(taiListForPaging.List, taiListforPagingItem)
	}

	pagingIEs.List = append(pagingIEs.List, ie)

	// Paging Priority (optional)
	if pagingPriority != nil {
		ie = ngapType.PagingIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPagingPriority
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PagingIEsPresentPagingPriority
		ie.Value.PagingPriority = pagingPriority
		pagingIEs.List = append(pagingIEs.List, ie)
	}

	// UE Radio Capability for Paging (optional)
	if ueRadioCapabilityForPaging != nil {
		ie = ngapType.PagingIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUERadioCapabilityForPaging
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PagingIEsPresentUERadioCapabilityForPaging
		ie.Value.UERadioCapabilityForPaging = new(ngapType.UERadioCapabilityForPaging)
		uERadioCapabilityForPaging := ie.Value.UERadioCapabilityForPaging
		if ueRadioCapabilityForPaging.NR != "" {
			uERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value, err = hex.DecodeString(ueRadioCapabilityForPaging.NR)
			if err != nil {
				return nil, fmt.Errorf("DecodeString ue.UeRadioCapabilityForPaging.NR error: %s", err)
			}
		}
		if ueRadioCapabilityForPaging.EUTRA != "" {
			uERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value, err = hex.DecodeString(ueRadioCapabilityForPaging.EUTRA)
			if err != nil {
				return nil, fmt.Errorf("DecodeString ue.UeRadioCapabilityForPaging.EUTRA error: %s", err)
			}
		}
		pagingIEs.List = append(pagingIEs.List, ie)
	}

	// Assistance Data for Paing (optional)
	if ueInfoOnRecommendedCellsAndRanNodesForPaging != nil {
		ie = ngapType.PagingIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAssistanceDataForPaging
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PagingIEsPresentAssistanceDataForPaging
		ie.Value.AssistanceDataForPaging = new(ngapType.AssistanceDataForPaging)

		assistanceDataForPaging := ie.Value.AssistanceDataForPaging
		assistanceDataForPaging.AssistanceDataForRecommendedCells = new(ngapType.AssistanceDataForRecommendedCells)
		recommendedCellList := &assistanceDataForPaging.
			AssistanceDataForRecommendedCells.RecommendedCellsForPaging.RecommendedCellList

		for _, recommendedCell := range ueInfoOnRecommendedCellsAndRanNodesForPaging.RecommendedCells {
			recommendedCellItem := ngapType.RecommendedCellItem{}
			switch recommendedCell.NgRanCGI.Present {
			case models.NgRanCgiPresentNRCGI:
				recommendedCellItem.NGRANCGI.Present = ngapType.NGRANCGIPresentNRCGI
				recommendedCellItem.NGRANCGI.NRCGI = new(ngapType.NRCGI)
				nrCGI := recommendedCellItem.NGRANCGI.NRCGI
				plmnID, err := util.PlmnIDToNgap(*recommendedCell.NgRanCGI.NRCGI.PlmnID)
				if err != nil {
					return nil, fmt.Errorf("error converting plmn id to ngap: %s", err)
				}
				nrCGI.PLMNIdentity = *plmnID
				nrCGI.NRCellIdentity.Value = ngapConvert.HexToBitString(recommendedCell.NgRanCGI.NRCGI.NrCellID, 36)
			case models.NgRanCgiPresentEUTRACGI:
				recommendedCellItem.NGRANCGI.Present = ngapType.NGRANCGIPresentEUTRACGI
				recommendedCellItem.NGRANCGI.EUTRACGI = new(ngapType.EUTRACGI)
				eutraCGI := recommendedCellItem.NGRANCGI.EUTRACGI
				plmnID, err := util.PlmnIDToNgap(*recommendedCell.NgRanCGI.NRCGI.PlmnID)
				if err != nil {
					return nil, fmt.Errorf("error converting plmn id to ngap: %s", err)
				}
				eutraCGI.PLMNIdentity = *plmnID
				eutraCGI.EUTRACellIdentity.Value = ngapConvert.HexToBitString(recommendedCell.NgRanCGI.EUTRACGI.EutraCellID, 28)
			}

			if recommendedCell.TimeStayedInCell != nil {
				recommendedCellItem.TimeStayedInCell = recommendedCell.TimeStayedInCell
			}
			recommendedCellList.List = append(recommendedCellList.List, recommendedCellItem)
		}

		pagingIEs.List = append(pagingIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}
