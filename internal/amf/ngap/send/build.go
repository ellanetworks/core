// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package send

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// nrEA / nrIA report whether the UE security capability value supports 5G-NR
// encryption / integrity algorithm n, as the 0/1 bit the NGAP UE Security
// Capabilities IE expects (TS 24.501 §9.11.3.54, TS 38.413).
func nrEA(sc *fgs.UESecurityCapability, n uint8) byte {
	if !sc.SupportsEA(n) {
		return 0
	}

	return 1
}

func nrIA(sc *fgs.UESecurityCapability, n uint8) byte {
	if !sc.SupportsIA(n) {
		return 0
	}

	return 1
}

func BuildPathSwitchRequestFailure(
	amfUeNgapID,
	ranUeNgapID int64,
	pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentUnsuccessfulOutcome
	pdu.UnsuccessfulOutcome = new(ngapType.UnsuccessfulOutcome)

	unsuccessfulOutcome := pdu.UnsuccessfulOutcome
	unsuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	unsuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	unsuccessfulOutcome.Value.Present = ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure
	unsuccessfulOutcome.Value.PathSwitchRequestFailure = new(ngapType.PathSwitchRequestFailure)

	pathSwitchRequestFailure := unsuccessfulOutcome.Value.PathSwitchRequestFailure
	pathSwitchRequestFailureIEs := &pathSwitchRequestFailure.ProtocolIEs

	ie := ngapType.PathSwitchRequestFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)

	ie = ngapType.PathSwitchRequestFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)

	if pduSessionResourceReleasedList != nil {
		ie = ngapType.PathSwitchRequestFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentPDUSessionResourceReleasedListPSFail
		ie.Value.PDUSessionResourceReleasedListPSFail = pduSessionResourceReleasedList
		pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)
	}

	if criticalityDiagnostics != nil {
		ie = ngapType.PathSwitchRequestFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

// Notifies peer CP NFs that this AMF and its GUAMI(s) are unavailable (TS 23.501).
func BuildDownlinkRanStatusTransfer(amfUENGAPID int64, ranUENGAPID int64, container *ngapType.RANStatusTransferTransparentContainer) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkRANStatusTransfer
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer
	initiatingMessage.Value.DownlinkRANStatusTransfer = new(ngapType.DownlinkRANStatusTransfer)

	downlinkRANStatusTransferIEs := &initiatingMessage.Value.DownlinkRANStatusTransfer.ProtocolIEs

	ie := ngapType.DownlinkRANStatusTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkRANStatusTransferIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)
	ie.Value.AMFUENGAPID.Value = amfUENGAPID
	downlinkRANStatusTransferIEs.List = append(downlinkRANStatusTransferIEs.List, ie)

	ie = ngapType.DownlinkRANStatusTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkRANStatusTransferIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)
	ie.Value.RANUENGAPID.Value = ranUENGAPID
	downlinkRANStatusTransferIEs.List = append(downlinkRANStatusTransferIEs.List, ie)

	ie = ngapType.DownlinkRANStatusTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANStatusTransferTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkRANStatusTransferIEsPresentRANStatusTransferTransparentContainer
	ie.Value.RANStatusTransferTransparentContainer = container
	downlinkRANStatusTransferIEs.List = append(downlinkRANStatusTransferIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildHandoverCancelAcknowledge(amfUENGAPID int64, ranUENGAPID int64) ([]byte, error) {
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

	ie := ngapType.HandoverCancelAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	handoverCancelAcknowledgeIEs.List = append(handoverCancelAcknowledgeIEs.List, ie)

	ie = ngapType.HandoverCancelAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverCancelAcknowledgeIEs.List = append(handoverCancelAcknowledgeIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildHandoverPreparationFailure(amfUENgapID int64, ranUENGAPID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) ([]byte, error) {
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

	ie := ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENgapID

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

	ie = ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

	ie = ngapType.HandoverPreparationFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentCriticalityDiagnostics
	ie.Value.Cause = new(ngapType.Cause)

	ie.Value.Cause = &cause

	handoverPreparationFailureIEs.List = append(handoverPreparationFailureIEs.List, ie)

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

// SMF-requested NG-RAN location reporting (TS 23.502, TS 23.501). The Location
// Reference ID To Be Cancelled IE is present only when the Event Type is "Stop UE presence in
// the area of interest".
func BuildLocationReportingControl(
	amfueNgapID int64,
	ranueNgapID int64,
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

	ie := ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfueNgapID

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	ie = ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranueNgapID

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	ie = ngapType.LocationReportingControlIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDLocationReportingRequestType
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.LocationReportingControlIEsPresentLocationReportingRequestType
	ie.Value.LocationReportingRequestType = new(ngapType.LocationReportingRequestType)

	locationReportingRequestType := ie.Value.LocationReportingRequestType

	locationReportingRequestType.EventType = eventType

	locationReportingRequestType.ReportArea.Value = ngapType.ReportAreaPresentCell // only cell-level reporting is supported

	if locationReportingRequestType.EventType.Value == ngapType.EventTypePresentStopUePresenceInAreaOfInterest {
		locationReportingRequestType.LocationReportingReferenceIDToBeCancelled = new(ngapType.LocationReportingReferenceID)
		locationReportingRequestType.LocationReportingReferenceIDToBeCancelled.Value = 0
	}

	locationReportingControlIEs.List = append(locationReportingControlIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildHandoverCommand(
	amfUENGAPID int64,
	ranUENGAPID int64,
	sourceUEhandoverType ngapType.HandoverType,
	pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList,
	pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd,
	container ngapType.TargetToSourceTransparentContainer,
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

	ie := ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCancelAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)

	handoverType := ie.Value.HandoverType
	handoverType.Value = sourceUEhandoverType.Value

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	if handoverType.Value == ngapType.HandoverTypePresentFivegsToEps {
		ie = ngapType.HandoverCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.HandoverCommandIEsPresentNASSecurityParametersFromNGRAN
		ie.Value.NASSecurityParametersFromNGRAN = new(ngapType.NASSecurityParametersFromNGRAN)

		handoverCommandIEs.List = append(handoverCommandIEs.List, ie)
	}

	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceHandoverList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverCommandIEsPresentPDUSessionResourceHandoverList
	ie.Value.PDUSessionResourceHandoverList = &pduSessionResourceHandoverList
	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	if len(pduSessionResourceToReleaseList.List) > 0 {
		ie = ngapType.HandoverCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.HandoverCommandIEsPresentPDUSessionResourceToReleaseListHOCmd
		ie.Value.PDUSessionResourceToReleaseListHOCmd = &pduSessionResourceToReleaseList
		handoverCommandIEs.List = append(handoverCommandIEs.List, ie)
	}

	ie = ngapType.HandoverCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTargetToSourceTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverCommandIEsPresentTargetToSourceTransparentContainer
	ie.Value.TargetToSourceTransparentContainer = &container

	handoverCommandIEs.List = append(handoverCommandIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildPathSwitchRequestAcknowledge(
	amfUeNgapID int64,
	ranUeNgapID int64,
	ueSecurityCapability *fgs.UESecurityCapability,
	ncc uint8,
	nh []byte,
	pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList,
	pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck,
	snssaiList []models.Snssai,
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

	ie := ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)

	ueSecurityCapabilities := ie.Value.UESecurityCapabilities
	nrEncryptionAlgorighm := []byte{0x00, 0x00}
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 1) << 7
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 2) << 6
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 3) << 5
	ueSecurityCapabilities.NRencryptionAlgorithms.Value = ngapConvert.ByteToBitString(nrEncryptionAlgorighm, 16)

	nrIntegrityAlgorithm := []byte{0x00, 0x00}
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 1) << 7
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 2) << 6
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 3) << 5
	ueSecurityCapabilities.NRintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(nrIntegrityAlgorithm, 16)

	// only support NR algorithms
	eutraEncryptionAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAencryptionAlgorithms.Value = ngapConvert.ByteToBitString(eutraEncryptionAlgorithm, 16)

	eutraIntegrityAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(eutraIntegrityAlgorithm, 16)

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityContext
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentSecurityContext
	ie.Value.SecurityContext = new(ngapType.SecurityContext)

	securityContext := ie.Value.SecurityContext
	securityContext.NextHopChainingCount.Value = int64(ncc)
	securityContext.NextHopNH.Value = ngapConvert.HexToBitString(hex.EncodeToString(nh), 256)

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSwitchedList
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentPDUSessionResourceSwitchedList
	ie.Value.PDUSessionResourceSwitchedList = &pduSessionResourceSwitchedList
	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	if len(pduSessionResourceReleasedList.List) > 0 {
		ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentPDUSessionResourceReleasedListPSAck
		ie.Value.PDUSessionResourceReleasedListPSAck = &pduSessionResourceReleasedList
		pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)
	}

	ie = ngapType.PathSwitchRequestAcknowledgeIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAllowedNSSAI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PathSwitchRequestAcknowledgeIEsPresentAllowedNSSAI
	ie.Value.AllowedNSSAI = new(ngapType.AllowedNSSAI)

	allowedNSSAI := ie.Value.AllowedNSSAI

	for i := range snssaiList {
		ngapSnssai, err := util.SNssaiToNgap(&snssaiList[i])
		if err != nil {
			return nil, fmt.Errorf("error converting snssai to ngap: %s", err)
		}

		allowedNSSAI.List = append(allowedNSSAI.List, ngapType.AllowedNSSAIItem{SNSSAI: ngapSnssai})
	}

	pathSwitchRequestAckIEs.List = append(pathSwitchRequestAckIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildHandoverRequest(
	amfUeNgapID int64,
	targetUEHandoverType ngapType.HandoverType,
	ambrUplink models.BitRate,
	ambrDownlink models.BitRate,
	ueSecurityCapability *fgs.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	snssaiList []models.Snssai,
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

	ie := ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDHandoverType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentHandoverType
	ie.Value.HandoverType = new(ngapType.HandoverType)

	handoverType := ie.Value.HandoverType
	handoverType.Value = targetUEHandoverType.Value

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.HandoverRequestIEsPresentCause
	ie.Value.Cause = &cause

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUEAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUEAggregateMaximumBitRate
	ie.Value.UEAggregateMaximumBitRate = new(ngapType.UEAggregateMaximumBitRate)

	ueAmbrUL := int64(ambrUplink.Bps())
	ueAmbrDL := int64(ambrDownlink.Bps())
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value = ueAmbrUL
	ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value = ueAmbrDL

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUESecurityCapabilities
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentUESecurityCapabilities
	ie.Value.UESecurityCapabilities = new(ngapType.UESecurityCapabilities)

	ueSecurityCapabilities := ie.Value.UESecurityCapabilities

	nrEncryptionAlgorighm := []byte{0x00, 0x00}
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 1) << 7
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 2) << 6
	nrEncryptionAlgorighm[0] |= nrEA(ueSecurityCapability, 3) << 5
	ueSecurityCapabilities.NRencryptionAlgorithms.Value = ngapConvert.ByteToBitString(nrEncryptionAlgorighm, 16)

	nrIntegrityAlgorithm := []byte{0x00, 0x00}
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 1) << 7
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 2) << 6
	nrIntegrityAlgorithm[0] |= nrIA(ueSecurityCapability, 3) << 5
	ueSecurityCapabilities.NRintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(nrIntegrityAlgorithm, 16)

	// only support NR algorithms
	eutraEncryptionAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAencryptionAlgorithms.Value = ngapConvert.ByteToBitString(eutraEncryptionAlgorithm, 16)

	eutraIntegrityAlgorithm := []byte{0x00, 0x00}
	ueSecurityCapabilities.EUTRAintegrityProtectionAlgorithms.Value = ngapConvert.ByteToBitString(eutraIntegrityAlgorithm, 16)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSecurityContext
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentSecurityContext
	ie.Value.SecurityContext = new(ngapType.SecurityContext)

	securityContext := ie.Value.SecurityContext
	securityContext.NextHopChainingCount.Value = int64(ncc)
	securityContext.NextHopNH.Value = ngapConvert.HexToBitString(hex.EncodeToString(nh), 256)

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentPDUSessionResourceSetupListHOReq
	ie.Value.PDUSessionResourceSetupListHOReq = &pduSessionResourceSetupListHOReq
	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAllowedNSSAI
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentAllowedNSSAI
	ie.Value.AllowedNSSAI = new(ngapType.AllowedNSSAI)

	allowedNSSAI := ie.Value.AllowedNSSAI

	for i := range snssaiList {
		ngapSnssai, err := util.SNssaiToNgap(&snssaiList[i])
		if err != nil {
			return nil, fmt.Errorf("error converting snssai to ngap: %s", err)
		}

		allowedNSSAI.List = append(allowedNSSAI.List, ngapType.AllowedNSSAIItem{SNSSAI: ngapSnssai})
	}

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)

	ie = ngapType.HandoverRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSourceToTargetTransparentContainer
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.HandoverRequestIEsPresentSourceToTargetTransparentContainer
	ie.Value.SourceToTargetTransparentContainer = new(ngapType.SourceToTargetTransparentContainer)

	sourceToTargetTransparentContaine := ie.Value.SourceToTargetTransparentContainer
	sourceToTargetTransparentContaine.Value = sourceToTargetTransparentContainer.Value

	handoverRequestIEs.List = append(handoverRequestIEs.List, ie)
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

// Paging Priority is included only when the N1N2MessageTransfer carries an ARP value
// for priority services, e.g. MPS or MCS (TS 23.502).
func AppendPDUSessionResourceSetupListHOReq(list *ngapType.PDUSessionResourceSetupListHOReq, pduSessionID uint8, snssai *models.Snssai, transfer []byte) {
	var item ngapType.PDUSessionResourceSetupItemHOReq

	item.PDUSessionID.Value = int64(pduSessionID)

	snssaiNgap, err := util.SNssaiToNgap(snssai)
	if err != nil {
		logger.AmfLog.Error("Convert SNssai to NGAP failed", zap.Error(err))
		return
	}

	item.SNSSAI = snssaiNgap
	item.HandoverRequestTransfer = transfer
	list.List = append(list.List, item)
}

func BuildDownlinkUEAssociatedNRPPaTransport(amfUENGAPID int64, ranUENGAPID int64, routingID int64, nrppaPdu []byte) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport
	initiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport = new(ngapType.DownlinkUEAssociatedNRPPaTransport)

	downlinkNRPPaTransport := initiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport
	downlinkNRPPaTransportIEs := &downlinkNRPPaTransport.ProtocolIEs

	ie := ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRoutingID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentRoutingID
	ie.Value.RoutingID = new(ngapType.RoutingID)

	rID := ie.Value.RoutingID
	rID.Value = aper.OctetString{byte((routingID >> 24) & 0xFF), byte((routingID >> 16) & 0xFF), byte((routingID >> 8) & 0xFF), byte(routingID & 0xFF)}

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	ie = ngapType.DownlinkUEAssociatedNRPPaTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNRPPaPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkUEAssociatedNRPPaTransportIEsPresentNRPPaPDU
	ie.Value.NRPPaPDU = new(ngapType.NRPPaPDU)

	nRPPaPDU := ie.Value.NRPPaPDU
	nRPPaPDU.Value = nrppaPdu

	downlinkNRPPaTransportIEs.List = append(downlinkNRPPaTransportIEs.List, ie)

	return ngap.Encoder(pdu)
}
