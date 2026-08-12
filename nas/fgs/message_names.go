// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "fmt"

// Message names from TS 24.501 table 9.7.1.
var messageTypeNames = map[MessageType]string{
	MsgRegistrationRequest:              "REGISTRATION REQUEST",
	MsgRegistrationAccept:               "REGISTRATION ACCEPT",
	MsgRegistrationComplete:             "REGISTRATION COMPLETE",
	MsgRegistrationReject:               "REGISTRATION REJECT",
	MsgDeregistrationRequestUEOrig:      "DEREGISTRATION REQUEST (UE originating)",
	MsgDeregistrationAcceptUEOrig:       "DEREGISTRATION ACCEPT (UE originating)",
	MsgDeregistrationRequestUETerm:      "DEREGISTRATION REQUEST (UE terminated)",
	MsgDeregistrationAcceptUETerm:       "DEREGISTRATION ACCEPT (UE terminated)",
	MsgServiceRequest:                   "SERVICE REQUEST",
	MsgServiceReject:                    "SERVICE REJECT",
	MsgServiceAccept:                    "SERVICE ACCEPT",
	MsgControlPlaneServiceRequest:       "CONTROL PLANE SERVICE REQUEST",
	MsgNetworkSliceSpecificAuthCommand:  "NETWORK SLICE-SPECIFIC AUTHENTICATION COMMAND",
	MsgNetworkSliceSpecificAuthComplete: "NETWORK SLICE-SPECIFIC AUTHENTICATION COMPLETE",
	MsgNetworkSliceSpecificAuthResult:   "NETWORK SLICE-SPECIFIC AUTHENTICATION RESULT",
	MsgConfigurationUpdateCommand:       "CONFIGURATION UPDATE COMMAND",
	MsgConfigurationUpdateComplete:      "CONFIGURATION UPDATE COMPLETE",
	MsgAuthenticationRequest:            "AUTHENTICATION REQUEST",
	MsgAuthenticationResponse:           "AUTHENTICATION RESPONSE",
	MsgAuthenticationReject:             "AUTHENTICATION REJECT",
	MsgAuthenticationFailure:            "AUTHENTICATION FAILURE",
	MsgAuthenticationResult:             "AUTHENTICATION RESULT",
	MsgIdentityRequest:                  "IDENTITY REQUEST",
	MsgIdentityResponse:                 "IDENTITY RESPONSE",
	MsgSecurityModeCommand:              "SECURITY MODE COMMAND",
	MsgSecurityModeComplete:             "SECURITY MODE COMPLETE",
	MsgSecurityModeReject:               "SECURITY MODE REJECT",
	MsgGMMStatus:                        "5GMM STATUS",
	MsgNotification:                     "NOTIFICATION",
	MsgNotificationResponse:             "NOTIFICATION RESPONSE",
	MsgULNASTransport:                   "UL NAS TRANSPORT",
	MsgDLNASTransport:                   "DL NAS TRANSPORT",
	MsgRelayKeyRequest:                  "RELAY KEY REQUEST",
	MsgRelayKeyAccept:                   "RELAY KEY ACCEPT",
	MsgRelayKeyReject:                   "RELAY KEY REJECT",
	MsgRelayAuthenticationRequest:       "RELAY AUTHENTICATION REQUEST",
	MsgRelayAuthenticationResponse:      "RELAY AUTHENTICATION RESPONSE",
}

// Name returns the message name TS 24.501 gives the type, or the empty string
// when the value is not one it assigns.
func (t MessageType) Name() string { return messageTypeNames[t] }

func (t MessageType) String() string {
	if name, ok := messageTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown message type (%#x)", uint8(t))
}

// Message names from TS 24.501 table 9.7.2.
var gsmMessageTypeNames = map[GSMMessageType]string{
	MsgPDUSessionEstablishmentRequest:   "PDU SESSION ESTABLISHMENT REQUEST",
	MsgPDUSessionEstablishmentAccept:    "PDU SESSION ESTABLISHMENT ACCEPT",
	MsgPDUSessionEstablishmentReject:    "PDU SESSION ESTABLISHMENT REJECT",
	MsgPDUSessionAuthenticationCommand:  "PDU SESSION AUTHENTICATION COMMAND",
	MsgPDUSessionAuthenticationComplete: "PDU SESSION AUTHENTICATION COMPLETE",
	MsgPDUSessionAuthenticationResult:   "PDU SESSION AUTHENTICATION RESULT",
	MsgPDUSessionModificationRequest:    "PDU SESSION MODIFICATION REQUEST",
	MsgPDUSessionModificationReject:     "PDU SESSION MODIFICATION REJECT",
	MsgPDUSessionModificationCommand:    "PDU SESSION MODIFICATION COMMAND",
	MsgPDUSessionModificationComplete:   "PDU SESSION MODIFICATION COMPLETE",
	MsgPDUSessionModificationCmdReject:  "PDU SESSION MODIFICATION COMMAND REJECT",
	MsgPDUSessionReleaseRequest:         "PDU SESSION RELEASE REQUEST",
	MsgPDUSessionReleaseReject:          "PDU SESSION RELEASE REJECT",
	MsgPDUSessionReleaseCommand:         "PDU SESSION RELEASE COMMAND",
	MsgPDUSessionReleaseComplete:        "PDU SESSION RELEASE COMPLETE",
	MsgGSMStatus:                        "5GSM STATUS",
	MsgServiceLevelAuthCommand:          "SERVICE-LEVEL-AA COMMAND",
	MsgServiceLevelAuthComplete:         "SERVICE-LEVEL-AA COMPLETE",
	MsgRemoteUEReport:                   "REMOTE UE REPORT",
	MsgRemoteUEReportResponse:           "REMOTE UE REPORT RESPONSE",
}

// Name returns the message name TS 24.501 gives the type, or the empty string
// when the value is not one it assigns.
func (t GSMMessageType) Name() string { return gsmMessageTypeNames[t] }

func (t GSMMessageType) String() string {
	if name, ok := gsmMessageTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown message type (%#x)", uint8(t))
}
