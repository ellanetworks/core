// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/s1ap"

// S1APProcedure is the human-readable S1AP message name recorded in the network
// event log.
type S1APProcedure string

const (
	S1APProcedureS1SetupRequest              S1APProcedure = "S1SetupRequest"
	S1APProcedureS1SetupResponse             S1APProcedure = "S1SetupResponse"
	S1APProcedureS1SetupFailure              S1APProcedure = "S1SetupFailure"
	S1APProcedureInitialUEMessage            S1APProcedure = "InitialUEMessage"
	S1APProcedureUplinkNASTransport          S1APProcedure = "UplinkNASTransport"
	S1APProcedureDownlinkNASTransport        S1APProcedure = "DownlinkNASTransport"
	S1APProcedureNASNonDeliveryIndication    S1APProcedure = "NASNonDeliveryIndication"
	S1APProcedureInitialContextSetupRequest  S1APProcedure = "InitialContextSetupRequest"
	S1APProcedureInitialContextSetupResponse S1APProcedure = "InitialContextSetupResponse"
	S1APProcedureInitialContextSetupFailure  S1APProcedure = "InitialContextSetupFailure"
	S1APProcedureUEContextReleaseRequest     S1APProcedure = "UEContextReleaseRequest"
	S1APProcedureUEContextReleaseCommand     S1APProcedure = "UEContextReleaseCommand"
	S1APProcedureUEContextReleaseComplete    S1APProcedure = "UEContextReleaseComplete"
	S1APProcedureUECapabilityInfoIndication  S1APProcedure = "UECapabilityInfoIndication"
	S1APProcedureErrorIndication             S1APProcedure = "ErrorIndication"
	S1APProcedureReset                       S1APProcedure = "Reset"
	S1APProcedureResetAcknowledge            S1APProcedure = "ResetAcknowledge"
	S1APProcedureENBConfigUpdate             S1APProcedure = "ENBConfigurationUpdate"
	S1APProcedureENBConfigUpdateAck          S1APProcedure = "ENBConfigurationUpdateAcknowledge"
	S1APProcedureENBConfigUpdateFailure      S1APProcedure = "ENBConfigurationUpdateFailure"
	S1APProcedureERABSetupRequest            S1APProcedure = "E-RABSetupRequest"
	S1APProcedureERABSetupResponse           S1APProcedure = "E-RABSetupResponse"
	S1APProcedureERABModifyRequest           S1APProcedure = "E-RABModifyRequest"
	S1APProcedureERABModifyResponse          S1APProcedure = "E-RABModifyResponse"
	S1APProcedureERABReleaseCommand          S1APProcedure = "E-RABReleaseCommand"
	S1APProcedureERABReleaseResponse         S1APProcedure = "E-RABReleaseResponse"
	S1APProcedurePaging                      S1APProcedure = "Paging"
	S1APProcedurePathSwitchRequest           S1APProcedure = "PathSwitchRequest"
	S1APProcedurePathSwitchRequestAck        S1APProcedure = "PathSwitchRequestAcknowledge"
	S1APProcedurePathSwitchRequestFailure    S1APProcedure = "PathSwitchRequestFailure"
	S1APProcedureHandoverRequired            S1APProcedure = "HandoverRequired"
	S1APProcedureHandoverRequest             S1APProcedure = "HandoverRequest"
	S1APProcedureHandoverRequestAck          S1APProcedure = "HandoverRequestAcknowledge"
	S1APProcedureHandoverFailure             S1APProcedure = "HandoverFailure"
	S1APProcedureHandoverCommand             S1APProcedure = "HandoverCommand"
	S1APProcedureHandoverPreparationFailure  S1APProcedure = "HandoverPreparationFailure"
	S1APProcedureHandoverNotify              S1APProcedure = "HandoverNotify"
	S1APProcedureHandoverCancel              S1APProcedure = "HandoverCancel"
	S1APProcedureHandoverCancelAcknowledge   S1APProcedure = "HandoverCancelAcknowledge"
	S1APProcedureENBStatusTransfer           S1APProcedure = "ENBStatusTransfer"
	S1APProcedureMMEStatusTransfer           S1APProcedure = "MMEStatusTransfer"
	S1APProcedureENBConfigurationTransfer    S1APProcedure = "ENBConfigurationTransfer"
	S1APProcedureMMEConfigurationTransfer    S1APProcedure = "MMEConfigurationTransfer"
	S1APProcedureUnknown                     S1APProcedure = "UnknownMessage"
)

// S1APMessageType resolves a decoded S1AP PDU to its message name. The S1AP
// message identity is the procedure code qualified by the PDU category, since a
// procedure spans request/response/failure (e.g. S1 Setup).
func S1APMessageType(pdu any) S1APProcedure {
	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		return s1apInitiatingMessageType(p.ProcedureCode)
	case *s1ap.SuccessfulOutcome:
		return s1apSuccessfulOutcomeType(p.ProcedureCode)
	case *s1ap.UnsuccessfulOutcome:
		return s1apUnsuccessfulOutcomeType(p.ProcedureCode)
	default:
		return S1APProcedureUnknown
	}
}

func s1apInitiatingMessageType(code s1ap.ProcedureCode) S1APProcedure {
	switch code {
	case s1ap.ProcS1Setup:
		return S1APProcedureS1SetupRequest
	case s1ap.ProcInitialUEMessage:
		return S1APProcedureInitialUEMessage
	case s1ap.ProcUplinkNASTransport:
		return S1APProcedureUplinkNASTransport
	case s1ap.ProcDownlinkNASTransport:
		return S1APProcedureDownlinkNASTransport
	case s1ap.ProcNASNonDeliveryIndication:
		return S1APProcedureNASNonDeliveryIndication
	case s1ap.ProcInitialContextSetup:
		return S1APProcedureInitialContextSetupRequest
	case s1ap.ProcUEContextReleaseRequest:
		return S1APProcedureUEContextReleaseRequest
	case s1ap.ProcUEContextRelease:
		return S1APProcedureUEContextReleaseCommand
	case s1ap.ProcUECapabilityInfoIndication:
		return S1APProcedureUECapabilityInfoIndication
	case s1ap.ProcErrorIndication:
		return S1APProcedureErrorIndication
	case s1ap.ProcReset:
		return S1APProcedureReset
	case s1ap.ProcENBConfigurationUpdate:
		return S1APProcedureENBConfigUpdate
	case s1ap.ProcERABSetup:
		return S1APProcedureERABSetupRequest
	case s1ap.ProcERABModify:
		return S1APProcedureERABModifyRequest
	case s1ap.ProcERABRelease:
		return S1APProcedureERABReleaseCommand
	case s1ap.ProcPathSwitchRequest:
		return S1APProcedurePathSwitchRequest
	case s1ap.ProcHandoverPreparation:
		return S1APProcedureHandoverRequired
	case s1ap.ProcHandoverResourceAllocation:
		return S1APProcedureHandoverRequest
	case s1ap.ProcHandoverNotification:
		return S1APProcedureHandoverNotify
	case s1ap.ProcHandoverCancel:
		return S1APProcedureHandoverCancel
	case s1ap.ProcENBStatusTransfer:
		return S1APProcedureENBStatusTransfer
	case s1ap.ProcMMEStatusTransfer:
		return S1APProcedureMMEStatusTransfer
	case s1ap.ProcENBConfigurationTransfer:
		return S1APProcedureENBConfigurationTransfer
	case s1ap.ProcMMEConfigurationTransfer:
		return S1APProcedureMMEConfigurationTransfer
	default:
		return S1APProcedureUnknown
	}
}

func s1apSuccessfulOutcomeType(code s1ap.ProcedureCode) S1APProcedure {
	switch code {
	case s1ap.ProcS1Setup:
		return S1APProcedureS1SetupResponse
	case s1ap.ProcInitialContextSetup:
		return S1APProcedureInitialContextSetupResponse
	case s1ap.ProcUEContextRelease:
		return S1APProcedureUEContextReleaseComplete
	case s1ap.ProcReset:
		return S1APProcedureResetAcknowledge
	case s1ap.ProcENBConfigurationUpdate:
		return S1APProcedureENBConfigUpdateAck
	case s1ap.ProcERABSetup:
		return S1APProcedureERABSetupResponse
	case s1ap.ProcERABModify:
		return S1APProcedureERABModifyResponse
	case s1ap.ProcERABRelease:
		return S1APProcedureERABReleaseResponse
	case s1ap.ProcPathSwitchRequest:
		return S1APProcedurePathSwitchRequestAck
	case s1ap.ProcHandoverPreparation:
		return S1APProcedureHandoverCommand
	case s1ap.ProcHandoverResourceAllocation:
		return S1APProcedureHandoverRequestAck
	case s1ap.ProcHandoverCancel:
		return S1APProcedureHandoverCancelAcknowledge
	default:
		return S1APProcedureUnknown
	}
}

func s1apUnsuccessfulOutcomeType(code s1ap.ProcedureCode) S1APProcedure {
	switch code {
	case s1ap.ProcS1Setup:
		return S1APProcedureS1SetupFailure
	case s1ap.ProcInitialContextSetup:
		return S1APProcedureInitialContextSetupFailure
	case s1ap.ProcENBConfigurationUpdate:
		return S1APProcedureENBConfigUpdateFailure
	case s1ap.ProcPathSwitchRequest:
		return S1APProcedurePathSwitchRequestFailure
	case s1ap.ProcHandoverPreparation:
		return S1APProcedureHandoverPreparationFailure
	case s1ap.ProcHandoverResourceAllocation:
		return S1APProcedureHandoverFailure
	default:
		return S1APProcedureUnknown
	}
}
