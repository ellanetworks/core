// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

// gmmMessageTypeNames is the authoritative set of 5GMM message types the AMF
// defines (TS 24.501 §9.7). It backs both the name lookup and gmmTypeDefined, so
// the two never drift.
var gmmMessageTypeNames = map[uint8]string{
	uint8(fgs.MsgRegistrationRequest):         "RegistrationRequest",
	uint8(fgs.MsgRegistrationAccept):          "RegistrationAccept",
	uint8(fgs.MsgRegistrationComplete):        "RegistrationComplete",
	uint8(fgs.MsgRegistrationReject):          "RegistrationReject",
	uint8(fgs.MsgDeregistrationRequestUEOrig): "DeregistrationRequestUEOriginatingDeregistration",
	uint8(fgs.MsgDeregistrationAcceptUEOrig):  "DeregistrationAcceptUEOriginatingDeregistration",
	uint8(fgs.MsgDeregistrationRequestUETerm): "DeregistrationRequestUETerminatedDeregistration",
	uint8(fgs.MsgDeregistrationAcceptUETerm):  "DeregistrationAcceptUETerminatedDeregistration",
	uint8(fgs.MsgServiceRequest):              "ServiceRequest",
	uint8(fgs.MsgServiceReject):               "ServiceReject",
	uint8(fgs.MsgServiceAccept):               "ServiceAccept",
	uint8(fgs.MsgConfigurationUpdateCommand):  "ConfigurationUpdateCommand",
	uint8(fgs.MsgConfigurationUpdateComplete): "ConfigurationUpdateComplete",
	uint8(fgs.MsgAuthenticationRequest):       "AuthenticationRequest",
	uint8(fgs.MsgAuthenticationResponse):      "AuthenticationResponse",
	uint8(fgs.MsgAuthenticationReject):        "AuthenticationReject",
	uint8(fgs.MsgAuthenticationFailure):       "AuthenticationFailure",
	uint8(fgs.MsgAuthenticationResult):        "AuthenticationResult",
	uint8(fgs.MsgIdentityRequest):             "IdentityRequest",
	uint8(fgs.MsgIdentityResponse):            "IdentityResponse",
	uint8(fgs.MsgSecurityModeCommand):         "SecurityModeCommand",
	uint8(fgs.MsgSecurityModeComplete):        "SecurityModeComplete",
	uint8(fgs.MsgSecurityModeReject):          "SecurityModeReject",
	uint8(fgs.MsgGMMStatus):                   "GMMStatus",
	uint8(fgs.MsgNotification):                "Notification",
	uint8(fgs.MsgNotificationResponse):        "NotificationResponse",
	uint8(fgs.MsgULNASTransport):              "ULNASTransport",
	uint8(fgs.MsgDLNASTransport):              "DLNASTransport",
}

func GmmMessageTypeName(code uint8) string {
	if name, ok := gmmMessageTypeNames[code]; ok {
		return name
	}

	return fmt.Sprintf("Unknown message type: 0x%02x", code)
}

// gmmUplinkTypes is the subset of 5GMM message types the AMF can receive from the UE
// (uplink or, for 5GMM STATUS, bidirectional). A downlink-only type received on the
// uplink is "not defined for the EPD in the given direction" (TS 24.501 §7.4 NOTE).
var gmmUplinkTypes = map[uint8]bool{
	uint8(fgs.MsgRegistrationRequest):         true,
	uint8(fgs.MsgRegistrationComplete):        true,
	uint8(fgs.MsgDeregistrationRequestUEOrig): true,
	uint8(fgs.MsgDeregistrationAcceptUETerm):  true,
	uint8(fgs.MsgServiceRequest):              true,
	uint8(fgs.MsgConfigurationUpdateComplete): true,
	uint8(fgs.MsgAuthenticationResponse):      true,
	uint8(fgs.MsgAuthenticationFailure):       true,
	uint8(fgs.MsgIdentityResponse):            true,
	uint8(fgs.MsgSecurityModeComplete):        true,
	uint8(fgs.MsgSecurityModeReject):          true,
	uint8(fgs.MsgGMMStatus):                   true,
	uint8(fgs.MsgNotificationResponse):        true,
	uint8(fgs.MsgULNASTransport):              true,
}

// gmmTypeDefined reports whether code is a 5GMM message type the AMF can receive from
// the UE. A downlink-only or unknown type draws 5GMM STATUS #97, not #96 (TS 24.501
// §7.4).
func gmmTypeDefined(code uint8) bool {
	return gmmUplinkTypes[code]
}
