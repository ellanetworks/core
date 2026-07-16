// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/free5gc/nas"
)

// gmmMessageTypeNames is the authoritative set of 5GMM message types the AMF
// defines (TS 24.501 §9.7). It backs both the name lookup and gmmTypeDefined, so
// the two never drift.
var gmmMessageTypeNames = map[uint8]string{
	nas.MsgTypeRegistrationRequest:                              "RegistrationRequest",
	nas.MsgTypeRegistrationAccept:                               "RegistrationAccept",
	nas.MsgTypeRegistrationComplete:                             "RegistrationComplete",
	nas.MsgTypeRegistrationReject:                               "RegistrationReject",
	nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration: "DeregistrationRequestUEOriginatingDeregistration",
	nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:  "DeregistrationAcceptUEOriginatingDeregistration",
	nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:  "DeregistrationRequestUETerminatedDeregistration",
	nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:   "DeregistrationAcceptUETerminatedDeregistration",
	nas.MsgTypeServiceRequest:                                   "ServiceRequest",
	nas.MsgTypeServiceReject:                                    "ServiceReject",
	nas.MsgTypeServiceAccept:                                    "ServiceAccept",
	nas.MsgTypeConfigurationUpdateCommand:                       "ConfigurationUpdateCommand",
	nas.MsgTypeConfigurationUpdateComplete:                      "ConfigurationUpdateComplete",
	nas.MsgTypeAuthenticationRequest:                            "AuthenticationRequest",
	nas.MsgTypeAuthenticationResponse:                           "AuthenticationResponse",
	nas.MsgTypeAuthenticationReject:                             "AuthenticationReject",
	nas.MsgTypeAuthenticationFailure:                            "AuthenticationFailure",
	nas.MsgTypeAuthenticationResult:                             "AuthenticationResult",
	nas.MsgTypeIdentityRequest:                                  "IdentityRequest",
	nas.MsgTypeIdentityResponse:                                 "IdentityResponse",
	nas.MsgTypeSecurityModeCommand:                              "SecurityModeCommand",
	nas.MsgTypeSecurityModeComplete:                             "SecurityModeComplete",
	nas.MsgTypeSecurityModeReject:                               "SecurityModeReject",
	nas.MsgTypeStatus5GMM:                                       "Status5GMM",
	nas.MsgTypeNotification:                                     "Notification",
	nas.MsgTypeNotificationResponse:                             "NotificationResponse",
	nas.MsgTypeULNASTransport:                                   "ULNASTransport",
	nas.MsgTypeDLNASTransport:                                   "DLNASTransport",
}

func GmmMessageTypeName(code uint8) string {
	if name, ok := gmmMessageTypeNames[code]; ok {
		return name
	}

	return fmt.Sprintf("Unknown message type: 0x%02x", code)
}

func gmmTypeDefined(code uint8) bool {
	_, ok := gmmMessageTypeNames[code]
	return ok
}
