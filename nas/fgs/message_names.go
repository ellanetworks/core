// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// String is the message name TS 24.501 table 9.7.1 gives the type.
func (t MessageType) String() string {
	switch t {
	case MsgRegistrationRequest:
		return "REGISTRATION REQUEST"
	case MsgRegistrationAccept:
		return "REGISTRATION ACCEPT"
	case MsgRegistrationComplete:
		return "REGISTRATION COMPLETE"
	case MsgRegistrationReject:
		return "REGISTRATION REJECT"
	case MsgDeregistrationRequestUEOrig:
		return "DEREGISTRATION REQUEST (UE originating)"
	case MsgDeregistrationAcceptUEOrig:
		return "DEREGISTRATION ACCEPT (UE originating)"
	case MsgDeregistrationRequestUETerm:
		return "DEREGISTRATION REQUEST (UE terminated)"
	case MsgDeregistrationAcceptUETerm:
		return "DEREGISTRATION ACCEPT (UE terminated)"
	case MsgServiceRequest:
		return "SERVICE REQUEST"
	case MsgServiceReject:
		return "SERVICE REJECT"
	case MsgServiceAccept:
		return "SERVICE ACCEPT"
	case MsgControlPlaneServiceRequest:
		return "CONTROL PLANE SERVICE REQUEST"
	case MsgNetworkSliceSpecificAuthCommand:
		return "NETWORK SLICE-SPECIFIC AUTHENTICATION COMMAND"
	case MsgNetworkSliceSpecificAuthComplete:
		return "NETWORK SLICE-SPECIFIC AUTHENTICATION COMPLETE"
	case MsgNetworkSliceSpecificAuthResult:
		return "NETWORK SLICE-SPECIFIC AUTHENTICATION RESULT"
	case MsgConfigurationUpdateCommand:
		return "CONFIGURATION UPDATE COMMAND"
	case MsgConfigurationUpdateComplete:
		return "CONFIGURATION UPDATE COMPLETE"
	case MsgAuthenticationRequest:
		return "AUTHENTICATION REQUEST"
	case MsgAuthenticationResponse:
		return "AUTHENTICATION RESPONSE"
	case MsgAuthenticationReject:
		return "AUTHENTICATION REJECT"
	case MsgAuthenticationFailure:
		return "AUTHENTICATION FAILURE"
	case MsgAuthenticationResult:
		return "AUTHENTICATION RESULT"
	case MsgIdentityRequest:
		return "IDENTITY REQUEST"
	case MsgIdentityResponse:
		return "IDENTITY RESPONSE"
	case MsgSecurityModeCommand:
		return "SECURITY MODE COMMAND"
	case MsgSecurityModeComplete:
		return "SECURITY MODE COMPLETE"
	case MsgSecurityModeReject:
		return "SECURITY MODE REJECT"
	case MsgGMMStatus:
		return "5GMM STATUS"
	case MsgNotification:
		return "NOTIFICATION"
	case MsgNotificationResponse:
		return "NOTIFICATION RESPONSE"
	case MsgULNASTransport:
		return "UL NAS TRANSPORT"
	case MsgDLNASTransport:
		return "DL NAS TRANSPORT"
	case MsgRelayKeyRequest:
		return "RELAY KEY REQUEST"
	case MsgRelayKeyAccept:
		return "RELAY KEY ACCEPT"
	case MsgRelayKeyReject:
		return "RELAY KEY REJECT"
	case MsgRelayAuthenticationRequest:
		return "RELAY AUTHENTICATION REQUEST"
	case MsgRelayAuthenticationResponse:
		return "RELAY AUTHENTICATION RESPONSE"
	default:
		return fmt.Sprintf("unknown message type (%#x)", uint8(t))
	}
}

// String is the message name TS 24.501 table 9.7.2 gives the type.
func (t GSMMessageType) String() string {
	switch t {
	case MsgPDUSessionEstablishmentRequest:
		return "PDU SESSION ESTABLISHMENT REQUEST"
	case MsgPDUSessionEstablishmentAccept:
		return "PDU SESSION ESTABLISHMENT ACCEPT"
	case MsgPDUSessionEstablishmentReject:
		return "PDU SESSION ESTABLISHMENT REJECT"
	case MsgPDUSessionAuthenticationCommand:
		return "PDU SESSION AUTHENTICATION COMMAND"
	case MsgPDUSessionAuthenticationComplete:
		return "PDU SESSION AUTHENTICATION COMPLETE"
	case MsgPDUSessionAuthenticationResult:
		return "PDU SESSION AUTHENTICATION RESULT"
	case MsgPDUSessionModificationRequest:
		return "PDU SESSION MODIFICATION REQUEST"
	case MsgPDUSessionModificationReject:
		return "PDU SESSION MODIFICATION REJECT"
	case MsgPDUSessionModificationCommand:
		return "PDU SESSION MODIFICATION COMMAND"
	case MsgPDUSessionModificationComplete:
		return "PDU SESSION MODIFICATION COMPLETE"
	case MsgPDUSessionModificationCmdReject:
		return "PDU SESSION MODIFICATION COMMAND REJECT"
	case MsgPDUSessionReleaseRequest:
		return "PDU SESSION RELEASE REQUEST"
	case MsgPDUSessionReleaseReject:
		return "PDU SESSION RELEASE REJECT"
	case MsgPDUSessionReleaseCommand:
		return "PDU SESSION RELEASE COMMAND"
	case MsgPDUSessionReleaseComplete:
		return "PDU SESSION RELEASE COMPLETE"
	case MsgGSMStatus:
		return "5GSM STATUS"
	case MsgServiceLevelAuthCommand:
		return "SERVICE-LEVEL-AA COMMAND"
	case MsgServiceLevelAuthComplete:
		return "SERVICE-LEVEL-AA COMPLETE"
	case MsgRemoteUEReport:
		return "REMOTE UE REPORT"
	case MsgRemoteUEReportResponse:
		return "REMOTE UE REPORT RESPONSE"
	default:
		return fmt.Sprintf("unknown message type (%#x)", uint8(t))
	}
}
