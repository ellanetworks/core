// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import "github.com/ellanetworks/core/nas/fgs"

func getGSMMessageName(msgType uint8) string {
	switch fgs.GSMMessageType(msgType) {
	case fgs.MsgPDUSessionEstablishmentRequest:
		return "PDU Session Establishment Request"
	case fgs.MsgPDUSessionEstablishmentAccept:
		return "PDU Session Establishment Accept"
	case fgs.MsgPDUSessionEstablishmentReject:
		return "PDU Session Establishment Reject"
	case fgs.MsgPDUSessionAuthenticationComplete:
		return "PDU Session Authentication Complete"
	case fgs.MsgPDUSessionModificationRequest:
		return "PDU Session Modification Request"
	case fgs.MsgPDUSessionModificationReject:
		return "PDU Session Modification Reject"
	case fgs.MsgPDUSessionModificationCommand:
		return "PDU Session Modification Command"
	case fgs.MsgPDUSessionModificationComplete:
		return "PDU Session Modification Complete"
	case fgs.MsgPDUSessionModificationCmdReject:
		return "PDU Session Modification Command Reject"
	case fgs.MsgPDUSessionReleaseRequest:
		return "PDU Session Release Request"
	case fgs.MsgPDUSessionReleaseCommand:
		return "PDU Session Release Command"
	case fgs.MsgPDUSessionReleaseComplete:
		return "PDU Session Release Complete"
	case fgs.MsgGSMStatus:
		return "5GSM Status"
	default:
		return "Unknown Message Type"
	}
}

func getGMMMessageName(msgType uint8) string {
	switch fgs.MessageType(msgType) {
	case fgs.MsgRegistrationRequest:
		return "Registration Request"
	case fgs.MsgRegistrationAccept:
		return "Registration Accept"
	case fgs.MsgRegistrationComplete:
		return "Registration Complete"
	case fgs.MsgRegistrationReject:
		return "Registration Reject"
	case fgs.MsgDeregistrationRequestUEOrig:
		return "Deregistration Request UE Originating Deregistration"
	case fgs.MsgDeregistrationAcceptUEOrig:
		return "Deregistration Accept UE Originating Deregistration"
	case fgs.MsgDeregistrationRequestUETerm:
		return "Deregistration Request UE Terminated Deregistration"
	case fgs.MsgDeregistrationAcceptUETerm:
		return "Deregistration Accept UE Terminated Deregistration"
	case fgs.MsgServiceRequest:
		return "Service Request"
	case fgs.MsgServiceReject:
		return "Service Reject"
	case fgs.MsgServiceAccept:
		return "Service Accept"
	case fgs.MsgConfigurationUpdateCommand:
		return "Configuration Update Command"
	case fgs.MsgConfigurationUpdateComplete:
		return "Configuration Update Complete"
	case fgs.MsgAuthenticationRequest:
		return "Authentication Request"
	case fgs.MsgAuthenticationResponse:
		return "Authentication Response"
	case fgs.MsgAuthenticationReject:
		return "Authentication Reject"
	case fgs.MsgAuthenticationFailure:
		return "Authentication Failure"
	case fgs.MsgAuthenticationResult:
		return "Authentication Result"
	case fgs.MsgIdentityRequest:
		return "Identity Request"
	case fgs.MsgIdentityResponse:
		return "Identity Response"
	case fgs.MsgSecurityModeCommand:
		return "Security Mode Command"
	case fgs.MsgSecurityModeComplete:
		return "Security Mode Complete"
	case fgs.MsgSecurityModeReject:
		return "Security Mode Reject"
	case fgs.MsgGMMStatus:
		return "5GMM Status"
	case fgs.MsgNotification:
		return "Notification"
	case fgs.MsgNotificationResponse:
		return "Notification Response"
	case fgs.MsgULNASTransport:
		return "UL NAS Transport"
	case fgs.MsgDLNASTransport:
		return "DL NAS Transport"
	default:
		return "Unknown Message Type"
	}
}
