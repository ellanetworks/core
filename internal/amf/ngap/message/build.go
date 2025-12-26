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

func BuildHandoverCancelAcknowledge(amfUENGAPID int64, ranUENGAPID int64, criticalityDiagnostics *ngapType.CriticalityDiagnostics) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverCancel
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge
	successfulOutcome.Value.HandoverCancelAcknowledge = new(ngapType.HandoverCancelAcknowledge)

	handoverCancelAcknowledge := successfulOutcome.Value.HandoverCancelAcknowledge
	handoverCancelAcknowledgeIEs := &handoverCancelAcknowledge.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.HandoverCancelAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	handoverCancelAcknowledgeIEs.List = append(handoverCancelAcknowledgeIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.HandoverCancelAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverCancelAcknowledgeIEs.List = append(handoverCancelAcknowledgeIEs.List, ie)

	// Criticality Diagnostics [optional]
	if criticalityDiagnostics != nil {
		ie := ngapType.HandoverCancelAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics

		handoverCancelAcknowledgeIEs.List = append(handoverCancelAcknowledgeIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

// nasPDU: from nas layer
// pduSessionResourceSetupRequestList: provided by AMF, and transfer data is from SMF
func BuildPDUSessionResourceSetupRequest(amfUENGAPID int64, ranUENGAPID int64, bitrateUplink string, bitrateDownlink string, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest
	initiatingMessage.Value.PDUSessionResourceSetupRequest = new(ngapType.PDUSessionResourceSetupRequest)

	pDUSessionResourceSetupRequest := initiatingMessage.Value.PDUSessionResourceSetupRequest
	pDUSessionResourceSetupRequestIEs := &pDUSessionResourceSetupRequest.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.PDUSessionResourceSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceSetupRequestIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	pDUSessionResourceSetupRequestIEs.List = append(pDUSessionResourceSetupRequestIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.PDUSessionResourceSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceSetupRequestIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	pDUSessionResourceSetupRequestIEs.List = append(pDUSessionResourceSetupRequestIEs.List, ie)

	// Ran Paging Priority (optional)

	// NAS-PDU (optional)
	if nasPdu != nil {
		ie = ngapType.PDUSessionResourceSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASPDU
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.PDUSessionResourceSetupRequestIEsPresentNASPDU
		ie.Value.NASPDU = new(ngapType.NASPDU)

		ie.Value.NASPDU.Value = nasPdu

		pDUSessionResourceSetupRequestIEs.List = append(pDUSessionResourceSetupRequestIEs.List, ie)
	}

	// PDU Session Resource Setup Request list
	ie = ngapType.PDUSessionResourceSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceSetupRequestIEsPresentPDUSessionResourceSetupListSUReq
	ie.Value.PDUSessionResourceSetupListSUReq = &pduSessionResourceSetupRequestList
	pDUSessionResourceSetupRequestIEs.List = append(pDUSessionResourceSetupRequestIEs.List, ie)

	// UE AggreateMaximum Bit Rate
	ie = ngapType.PDUSessionResourceSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceSetupRequestIEsPresentUEAggregateMaximumBitRate
	ie.Value.UEAggregateMaximumBitRate = new(ngapType.UEAggregateMaximumBitRate)
	ueAmbrUL := ngapConvert.UEAmbrToInt64(bitrateUplink)
	ueAmbrDL := ngapConvert.UEAmbrToInt64(bitrateDownlink)
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value = ueAmbrUL
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value = ueAmbrDL
	pDUSessionResourceSetupRequestIEs.List = append(pDUSessionResourceSetupRequestIEs.List, ie)

	return ngap.Encoder(pdu)
}

// pduSessionResourceModifyConfirmList: provided by AMF, and transfer data is return from SMF
// pduSessionResourceFailedToModifyList: provided by AMF, and transfer data is return from SMF
func BuildPDUSessionResourceModifyConfirm(
	amfUENGAPID int64,
	ranUENGAPID int64,
	pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm,
	pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceModifyIndication
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm
	successfulOutcome.Value.PDUSessionResourceModifyConfirm = new(ngapType.PDUSessionResourceModifyConfirm)

	pDUSessionResourceModifyConfirm := successfulOutcome.Value.PDUSessionResourceModifyConfirm
	pDUSessionResourceModifyConfirmIEs := &pDUSessionResourceModifyConfirm.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.PDUSessionResourceModifyConfirmIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceModifyConfirmIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	pDUSessionResourceModifyConfirmIEs.List = append(pDUSessionResourceModifyConfirmIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.PDUSessionResourceModifyConfirmIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceModifyConfirmIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	pDUSessionResourceModifyConfirmIEs.List = append(pDUSessionResourceModifyConfirmIEs.List, ie)

	// PDU Session Resource Modify Confirm List
	ie = ngapType.PDUSessionResourceModifyConfirmIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PDUSessionResourceModifyConfirmIEsPresentPDUSessionResourceModifyListModCfm
	ie.Value.PDUSessionResourceModifyListModCfm = &pduSessionResourceModifyConfirmList
	pDUSessionResourceModifyConfirmIEs.List = append(pDUSessionResourceModifyConfirmIEs.List, ie)

	// PDU Session Resource Failed to Modify List
	if len(pduSessionResourceFailedToModifyList.List) > 0 {
		ie = ngapType.PDUSessionResourceModifyConfirmIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceModifyConfirmIEsPresentPDUSessionResourceFailedToModifyListModCfm
		ie.Value.PDUSessionResourceFailedToModifyListModCfm = &pduSessionResourceFailedToModifyList
		pDUSessionResourceModifyConfirmIEs.List = append(pDUSessionResourceModifyConfirmIEs.List, ie)
	}

	// Criticality Diagnostics (optional)
	if criticalityDiagnostics != nil {
		ie = ngapType.PDUSessionResourceModifyConfirmIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceModifyConfirmIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		pDUSessionResourceModifyConfirmIEs.List = append(pDUSessionResourceModifyConfirmIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildInitialContextSetupRequest(
	amfUENgapID int64,
	ranUENgapID int64,
	bitrateUplink string,
	bitrateDownlink string,
	allowedNssai *models.Snssai,
	kgnodeb []byte,
	servingPlmnID models.PlmnID,
	radioCapability string,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *nasType.UESecurityCapability,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) ([]byte, error) {
	// Old AMF: new amf should get old amf's amf name

	// rrcInactiveTransitionReportRequest: configured by amf
	// This IE is used to request the NG-RAN node to report or stop reporting to the 5GC
	// when the UE enters or leaves RRC_INACTIVE state. (TS 38.413 9.3.1.91)

	// accessType indicate amfUe send this msg for which accessType
	// emergencyFallbackIndicator: configured by amf (TS 23.501 5.16.4.11)
	// coreNetworkAssistanceInfo TS 23.501 5.4.6, 5.4.6.2

	// Mobility Restriction List TS 23.501 5.3.4
	// TS 23.501 5.3.4.1.1: For a given UE, the core network determines the Mobility restrictions
	// based on UE subscription information.
	// TS 38.413 9.3.1.85: This IE defines roaming or access restrictions for subsequent mobility action for
	// which the NR-RAN provides information about the target of the mobility action towards
	// the UE, e.g., handover, or for SCG selection during dual connectivity operation or for
	// assigning proper RNAs. If the NG-RAN receives the Mobility Restriction List IE, it shall
	// overwrite previously received mobility restriction information.

	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeInitialContextSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentInitialContextSetupRequest
	initiatingMessage.Value.InitialContextSetupRequest = new(ngapType.InitialContextSetupRequest)

	initialContextSetupRequest := initiatingMessage.Value.InitialContextSetupRequest
	initialContextSetupRequestIEs := &initialContextSetupRequest.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENgapID

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENgapID

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// UE Aggregate Maximum Bit Rate (conditional: if pdu session resource setup)
	// The subscribed UE-AMBR is a subscription parameter which is
	// retrieved from UDM and provided to the (R)AN by the AMF
	if pduSessionResourceSetupRequestList != nil {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUEAggregateMaximumBitRate
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentUEAggregateMaximumBitRate
		ie.Value.UEAggregateMaximumBitRate = new(ngapType.UEAggregateMaximumBitRate)

		ueAmbrUL := ngapConvert.UEAmbrToInt64(bitrateUplink)
		ueAmbrDL := ngapConvert.UEAmbrToInt64(bitrateDownlink)
		ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value = ueAmbrUL
		ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value = ueAmbrDL

		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}

	// GUAMI
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGUAMI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentGUAMI
	ie.Value.GUAMI = new(ngapType.GUAMI)

	guami := ie.Value.GUAMI
	plmnID := &guami.PLMNIdentity
	amfRegionID := &guami.AMFRegionID
	amfSetID := &guami.AMFSetID
	amfPtrID := &guami.AMFPointer

	ngapPlmnID, err := util.PlmnIDToNgap(*supportedGUAMI.PlmnID)
	if err != nil {
		return nil, fmt.Errorf("cannot convert PlmnID to ngap PlmnID: %+v", err)
	}
	*plmnID = *ngapPlmnID
	amfRegionID.Value, amfSetID.Value, amfPtrID.Value = ngapConvert.AmfIdToNgap(supportedGUAMI.AmfID)

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// PDU Session Resource Setup Request List
	if pduSessionResourceSetupRequestList != nil && len(pduSessionResourceSetupRequestList.List) > 0 {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentPDUSessionResourceSetupListCxtReq
		ie.Value.PDUSessionResourceSetupListCxtReq = pduSessionResourceSetupRequestList
		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}

	// Allowed NSSAI
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAllowedNSSAI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentAllowedNSSAI
	ie.Value.AllowedNSSAI = new(ngapType.AllowedNSSAI)

	allowedNSSAI := ie.Value.AllowedNSSAI

	snssaiNgap, err := util.SNssaiToNgap(allowedNssai)
	if err != nil {
		return nil, fmt.Errorf("error converting SNssai to NGAP: %+v", err)
	}

	allowedNSSAIItem := ngapType.AllowedNSSAIItem{}
	allowedNSSAIItem.SNSSAI = snssaiNgap
	allowedNSSAI.List = append(allowedNSSAI.List, allowedNSSAIItem)

	if len(allowedNSSAI.List) == 0 {
		return nil, fmt.Errorf("allowed NSSAI list is empty")
	}

	if len(allowedNSSAI.List) > 8 {
		return nil, fmt.Errorf("length of allowed NSSAI list exceeds 8")
	}

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// UE Security Capabilities
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)

	ueSecurityCapabilities := ie.Value.UESecurityCapabilities
	nrEncryptionAlgorighm := []byte{0x00, 0x00}

	if ueSecurityCapability == nil {
		return nil, fmt.Errorf("UE Security Capability is nil")
	}

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

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// Security Key
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityKey
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentSecurityKey
	ie.Value.SecurityKey = new(ngapType.SecurityKey)

	securityKey := ie.Value.SecurityKey
	securityKey.Value = ngapConvert.ByteToBitString(kgnodeb, 256)

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// Mobility Restriction List (optional)
	ie = ngapType.InitialContextSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDMobilityRestrictionList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentMobilityRestrictionList
	ie.Value.MobilityRestrictionList = new(ngapType.MobilityRestrictionList)

	mobilityRestrictionList, err := BuildIEMobilityRestrictionList(servingPlmnID)
	if err != nil {
		return nil, fmt.Errorf("error building Mobility Restriction List IE: %s", err)
	}

	ie.Value.MobilityRestrictionList = mobilityRestrictionList

	initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)

	// UE Radio Capability (optional)
	if radioCapability != "" {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUERadioCapability
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentUERadioCapability
		ie.Value.UERadioCapability = new(ngapType.UERadioCapability)
		uecapa, err := hex.DecodeString(radioCapability)
		if err != nil {
			return nil, fmt.Errorf("cannot decode UeRadioCapability: %+v", err)
		}
		ie.Value.UERadioCapability.Value = uecapa
		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}

	// Masked IMEISV (optional)
	// TS 38.413 9.3.1.54; TS 23.003 6.2; TS 23.501 5.9.3
	// last 4 digits of the SNR masked by setting the corresponding bits to 1.
	// The first to fourth bits correspond to the first digit of the IMEISV,
	// the fifth to eighth bits correspond to the second digit of the IMEISV, and so on
	/*if amfUe.Pei != "" && strings.HasPrefix(amfUe.Pei, "imeisv") {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDMaskedIMEISV
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentMaskedIMEISV
		ie.Value.MaskedIMEISV = new(ngapType.MaskedIMEISV)

		imeisv := strings.TrimPrefix(amfUe.Pei, "imeisv-")
		imeisvBytes, err := hex.DecodeString(imeisv)
		if err != nil {
			logger.AmfLog.Errorf("[Build Error] DecodeString imeisv error: %+v", err)
		}

		var maskedImeisv []byte
		maskedImeisv = append(maskedImeisv, imeisvBytes[:5]...)
		maskedImeisv = append(maskedImeisv, []byte{0xff, 0xff}...)
		maskedImeisv = append(maskedImeisv, imeisvBytes[7])
		ie.Value.MaskedIMEISV.Value = aper.BitString{
			BitLength: 64,
			Bytes:     maskedImeisv,
		}
		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}*/

	// NAS-PDU (optional)
	if nasPdu != nil {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASPDU
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentNASPDU
		ie.Value.NASPDU = new(ngapType.NASPDU)

		ie.Value.NASPDU.Value = nasPdu

		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}

	// UE Radio Capability for Paging (optional)
	if ueRadioCapabilityForPaging != nil {
		ie = ngapType.InitialContextSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUERadioCapabilityForPaging
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.InitialContextSetupRequestIEsPresentUERadioCapabilityForPaging
		ie.Value.UERadioCapabilityForPaging = new(ngapType.UERadioCapabilityForPaging)
		uERadioCapabilityForPaging := ie.Value.UERadioCapabilityForPaging
		var err error
		if ueRadioCapabilityForPaging.NR != "" {
			uERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value, err = hex.DecodeString(ueRadioCapabilityForPaging.NR)
			if err != nil {
				return nil, fmt.Errorf("DecodeString amfUe.UeRadioCapabilityForPaging.NR error: %+v", err)
			}
		}
		if ueRadioCapabilityForPaging.EUTRA != "" {
			uERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value, err = hex.DecodeString(ueRadioCapabilityForPaging.EUTRA)
			if err != nil {
				return nil, fmt.Errorf("DecodeString amfUe.UeRadioCapabilityForPaging.EUTRA error: %+v", err)
			}
		}
		initialContextSetupRequestIEs.List = append(initialContextSetupRequestIEs.List, ie)
	}

	byteMsg, err := ngap.Encoder(pdu)
	if err != nil {
		return nil, fmt.Errorf("could not encode ngap message: %+v", err)
	}

	return byteMsg, nil
}

// pduSessionResourceHandoverList: provided by amf and transfer is return from smf
// pduSessionResourceToReleaseList: provided by amf and transfer is return from smf
// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
// when received node can't comprehend the IE or missing IE
func BuildHandoverCommand(
	amfUENGAPID int64,
	ranUENGAPID int64,
	sourceUEhandoverType ngapType.HandoverType,
	pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList,
	pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd,
	container ngapType.TargetToSourceTransparentContainer,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentHandoverCommand
	successfulOutcome.Value.HandoverCommand = new(ngapType.HandoverCommand)

	handoverCommand := successfulOutcome.Value.HandoverCommand
	handoverCommandIEs := &handoverCommand.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	// Handover Type
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)

	handoverType := ie.Value.HandoverType
	handoverType.Value = sourceUEhandoverType.Value

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	// NAS Security Parameters from NG-RAN [C-iftoEPS]
	if handoverType.Value == ngapType.HandoverTypePresentFivegsToEps {
		ie = ngapType.HandoverCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverCommandIEsPresentNASSecurityParametersFromNGRAN
		ie.Value.NASSecurityParametersFromNGRAN = new(ngapType.NASSecurityParametersFromNGRAN)

		handoverCommandIEs.List = append(handoverCommandIEs.List, ie)
	}

	// PDU Session Resource Handover List
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceHandoverList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCommandIEsPresentPDUSessionResourceHandoverList
	ie.Value.PDUSessionResourceHandoverList = &pduSessionResourceHandoverList
	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	// PDU Session Resource to Release List
	if len(pduSessionResourceToReleaseList.List) > 0 {
		ie = ngapType.HandoverCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverCommandIEsPresentPDUSessionResourceToReleaseListHOCmd
		ie.Value.PDUSessionResourceToReleaseListHOCmd = &pduSessionResourceToReleaseList
		handoverCommandIEs.List = append(handoverCommandIEs.List, ie)
	}

	// Target to Source Transparent Container
	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTargetToSourceTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentTargetToSourceTransparentContainer
	ie.Value.TargetToSourceTransparentContainer = &container

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	// Criticality Diagnostics [optional]
	if criticalityDiagnostics != nil {
		ie := ngapType.HandoverCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics

		handoverCommandIEs.List = append(handoverCommandIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildHandoverPreparationFailure(amfUENgapID int64, ranUENGAPID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) ([]byte, error) {
	// cause = initiate the Handover Cancel procedure with the appropriate value for the Cause IE.

	// criticalityDiagnostics = criticalityDiagonstics IE in receiver node's error indication
	// when received node can't comprehend the IE or missing IE

	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentUnsuccessfulOutcome
	pdu.UnsuccessfulOutcome = new(ngapType.UnsuccessfulOutcome)

	unsuccessfulOutcome := pdu.UnsuccessfulOutcome
	unsuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeHandoverPreparation
	unsuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	unsuccessfulOutcome.Value.Present = ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure
	unsuccessfulOutcome.Value.HandoverPreparationFailure = new(ngapType.HandoverPreparationFailure)

	handoverPreparationFailure := unsuccessfulOutcome.Value.HandoverPreparationFailure
	handoverPreparationFailureIEs := &handoverPreparationFailure.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENgapID

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

	// Cause
	ie = ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentCriticalityDiagnostics
	ie.Value.Cause = new(ngapType.Cause)

	ie.Value.Cause = &cause

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

	// Criticality Diagnostics [optional]
	if criticalityDiagnostics != nil {
		ie := ngapType.HandoverPreparationFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics

		handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)
	}

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

// AOI List is from SMF
// The SMF may subscribe to the UE mobility event notification from the AMF
// (e.g. location reporting, UE moving into or out of Area Of Interest) TS 23.502 4.3.2.2.1 Step.17
// The Location Reporting Control message shall identify the UE for which reports are requested and
// may include Reporting Type, Location Reporting Level, Area Of Interest and
// Request Reference ID TS 23.502 4.10 LocationReportingProcedure
// The AMF may request the NG-RAN location reporting with event reporting type
// (e.g. UE location or UE presence in Area of Interest),
// reporting mode and its related parameters (e.g. number of reporting) TS 23.501 5.4.7
// Location Reference ID To Be Cancelled IE shall be present if
// the Event Type IE is set to "Stop UE presence in the area of interest".
func BuildLocationReportingControl(
	amfueNgapID int64,
	ranueNgapID int64,
	AOIList *ngapType.AreaOfInterestList,
	LocationReportingReferenceIDToBeCancelled int64,
	eventType ngapType.EventType,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeLocationReportingControl
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentLocationReportingControl
	initiatingMessage.Value.LocationReportingControl = new(ngapType.LocationReportingControl)

	locationReportingControl := initiatingMessage.Value.LocationReportingControl
	locationReportingControlIEs := &locationReportingControl.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfueNgapID

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranueNgapID

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	// Location Reporting Request Type
	ie = ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDLocationReportingRequestType
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentLocationReportingRequestType
	ie.Value.LocationReportingRequestType = new(ngapType.LocationReportingRequestType)

	locationReportingRequestType := ie.Value.LocationReportingRequestType

	// Event Type
	locationReportingRequestType.EventType = eventType

	// Report Area in Location Reporting Request Type
	locationReportingRequestType.ReportArea.Value = ngapType.ReportAreaPresentCell // only this enum

	// AOI List in Location Reporting Request Type
	if AOIList != nil {
		locationReportingRequestType.AreaOfInterestList = new(ngapType.AreaOfInterestList)
		areaOfInterestList := locationReportingRequestType.AreaOfInterestList
		areaOfInterestList.List = AOIList.List
	}

	// location reference ID to be Cancelled [Conditional]
	if locationReportingRequestType.EventType.Value ==
		ngapType.EventTypePresentStopUePresenceInAreaOfInterest {
		locationReportingRequestType.LocationReportingReferenceIDToBeCancelled = new(ngapType.LocationReportingReferenceID)
		locationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value = LocationReportingReferenceIDToBeCancelled
	}

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	return ngap.Encoder(pdu)
}
