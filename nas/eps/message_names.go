// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// Message names from TS 24.301 table 9.8.1.
var messageTypeNames = map[MessageType]string{
	MsgAttachRequest:               "ATTACH REQUEST",
	MsgAttachAccept:                "ATTACH ACCEPT",
	MsgAttachComplete:              "ATTACH COMPLETE",
	MsgAttachReject:                "ATTACH REJECT",
	MsgDetachRequest:               "DETACH REQUEST",
	MsgDetachAccept:                "DETACH ACCEPT",
	MsgTrackingAreaUpdateRequest:   "TRACKING AREA UPDATE REQUEST",
	MsgTrackingAreaUpdateAccept:    "TRACKING AREA UPDATE ACCEPT",
	MsgTrackingAreaUpdateComplete:  "TRACKING AREA UPDATE COMPLETE",
	MsgTrackingAreaUpdateReject:    "TRACKING AREA UPDATE REJECT",
	MsgExtendedServiceRequest:      "EXTENDED SERVICE REQUEST",
	MsgControlPlaneServiceRequest:  "CONTROL PLANE SERVICE REQUEST",
	MsgServiceReject:               "SERVICE REJECT",
	MsgServiceAccept:               "SERVICE ACCEPT",
	MsgGUTIReallocationCommand:     "GUTI REALLOCATION COMMAND",
	MsgGUTIReallocationComplete:    "GUTI REALLOCATION COMPLETE",
	MsgAuthenticationRequest:       "AUTHENTICATION REQUEST",
	MsgAuthenticationResponse:      "AUTHENTICATION RESPONSE",
	MsgAuthenticationReject:        "AUTHENTICATION REJECT",
	MsgIdentityRequest:             "IDENTITY REQUEST",
	MsgIdentityResponse:            "IDENTITY RESPONSE",
	MsgAuthenticationFailure:       "AUTHENTICATION FAILURE",
	MsgSecurityModeCommand:         "SECURITY MODE COMMAND",
	MsgSecurityModeComplete:        "SECURITY MODE COMPLETE",
	MsgSecurityModeReject:          "SECURITY MODE REJECT",
	MsgEMMStatus:                   "EMM STATUS",
	MsgEMMInformation:              "EMM INFORMATION",
	MsgDownlinkNASTransport:        "DOWNLINK NAS TRANSPORT",
	MsgUplinkNASTransport:          "UPLINK NAS TRANSPORT",
	MsgCSServiceNotification:       "CS SERVICE NOTIFICATION",
	MsgDownlinkGenericNASTransport: "DOWNLINK GENERIC NAS TRANSPORT",
	MsgUplinkGenericNASTransport:   "UPLINK GENERIC NAS TRANSPORT",
}

// Name returns the message name TS 24.301 gives the type, or the empty string
// when the value is not one it assigns.
func (t MessageType) Name() string { return messageTypeNames[t] }

func (t MessageType) String() string {
	if name, ok := messageTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown message type (%#x)", uint8(t))
}

// Message names from TS 24.301 table 9.8.2.
//
// #nosec G101 -- these are 3GPP message names, not credentials. G101 matches
// its "bearer" pattern against the EPS bearer context messages, where a bearer
// is an EPS radio bearer (TS 24.301 §6.1).
var esmMessageTypeNames = map[ESMMessageType]string{
	MsgActivateDefaultEPSBearerContextRequest:   "ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST",
	MsgActivateDefaultEPSBearerContextAccept:    "ACTIVATE DEFAULT EPS BEARER CONTEXT ACCEPT",
	MsgActivateDefaultEPSBearerContextReject:    "ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT",
	MsgActivateDedicatedEPSBearerContextRequest: "ACTIVATE DEDICATED EPS BEARER CONTEXT REQUEST",
	MsgActivateDedicatedEPSBearerContextAccept:  "ACTIVATE DEDICATED EPS BEARER CONTEXT ACCEPT",
	MsgActivateDedicatedEPSBearerContextReject:  "ACTIVATE DEDICATED EPS BEARER CONTEXT REJECT",
	MsgModifyEPSBearerContextRequest:            "MODIFY EPS BEARER CONTEXT REQUEST",
	MsgModifyEPSBearerContextAccept:             "MODIFY EPS BEARER CONTEXT ACCEPT",
	MsgModifyEPSBearerContextReject:             "MODIFY EPS BEARER CONTEXT REJECT",
	MsgDeactivateEPSBearerContextRequest:        "DEACTIVATE EPS BEARER CONTEXT REQUEST",
	MsgDeactivateEPSBearerContextAccept:         "DEACTIVATE EPS BEARER CONTEXT ACCEPT",
	MsgPDNConnectivityRequest:                   "PDN CONNECTIVITY REQUEST",
	MsgPDNConnectivityReject:                    "PDN CONNECTIVITY REJECT",
	MsgPDNDisconnectRequest:                     "PDN DISCONNECT REQUEST",
	MsgPDNDisconnectReject:                      "PDN DISCONNECT REJECT",
	MsgBearerResourceAllocationRequest:          "BEARER RESOURCE ALLOCATION REQUEST",
	MsgBearerResourceAllocationReject:           "BEARER RESOURCE ALLOCATION REJECT",
	MsgBearerResourceModificationRequest:        "BEARER RESOURCE MODIFICATION REQUEST",
	MsgBearerResourceModificationReject:         "BEARER RESOURCE MODIFICATION REJECT",
	MsgESMInformationRequest:                    "ESM INFORMATION REQUEST",
	MsgESMInformationResponse:                   "ESM INFORMATION RESPONSE",
	MsgNotification:                             "NOTIFICATION",
	MsgESMDummyMessage:                          "ESM DUMMY MESSAGE",
	MsgESMStatus:                                "ESM STATUS",
	MsgRemoteUEReport:                           "REMOTE UE REPORT",
	MsgRemoteUEReportResponse:                   "REMOTE UE REPORT RESPONSE",
	MsgESMDataTransport:                         "ESM DATA TRANSPORT",
}

// Name returns the message name TS 24.301 gives the type, or the empty string
// when the value is not one it assigns.
func (t ESMMessageType) Name() string { return esmMessageTypeNames[t] }

func (t ESMMessageType) String() string {
	if name, ok := esmMessageTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown message type (%#x)", uint8(t))
}
