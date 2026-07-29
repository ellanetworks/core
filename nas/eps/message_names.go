// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// String is the message name TS 24.301 table 9.8.1 gives the type.
func (t MessageType) String() string {
	switch t {
	case MsgAttachRequest:
		return "ATTACH REQUEST"
	case MsgAttachAccept:
		return "ATTACH ACCEPT"
	case MsgAttachComplete:
		return "ATTACH COMPLETE"
	case MsgAttachReject:
		return "ATTACH REJECT"
	case MsgDetachRequest:
		return "DETACH REQUEST"
	case MsgDetachAccept:
		return "DETACH ACCEPT"
	case MsgTrackingAreaUpdateRequest:
		return "TRACKING AREA UPDATE REQUEST"
	case MsgTrackingAreaUpdateAccept:
		return "TRACKING AREA UPDATE ACCEPT"
	case MsgTrackingAreaUpdateComplete:
		return "TRACKING AREA UPDATE COMPLETE"
	case MsgTrackingAreaUpdateReject:
		return "TRACKING AREA UPDATE REJECT"
	case MsgExtendedServiceRequest:
		return "EXTENDED SERVICE REQUEST"
	case MsgControlPlaneServiceRequest:
		return "CONTROL PLANE SERVICE REQUEST"
	case MsgServiceReject:
		return "SERVICE REJECT"
	case MsgServiceAccept:
		return "SERVICE ACCEPT"
	case MsgGUTIReallocationCommand:
		return "GUTI REALLOCATION COMMAND"
	case MsgGUTIReallocationComplete:
		return "GUTI REALLOCATION COMPLETE"
	case MsgAuthenticationRequest:
		return "AUTHENTICATION REQUEST"
	case MsgAuthenticationResponse:
		return "AUTHENTICATION RESPONSE"
	case MsgAuthenticationReject:
		return "AUTHENTICATION REJECT"
	case MsgIdentityRequest:
		return "IDENTITY REQUEST"
	case MsgIdentityResponse:
		return "IDENTITY RESPONSE"
	case MsgAuthenticationFailure:
		return "AUTHENTICATION FAILURE"
	case MsgSecurityModeCommand:
		return "SECURITY MODE COMMAND"
	case MsgSecurityModeComplete:
		return "SECURITY MODE COMPLETE"
	case MsgSecurityModeReject:
		return "SECURITY MODE REJECT"
	case MsgEMMStatus:
		return "EMM STATUS"
	case MsgEMMInformation:
		return "EMM INFORMATION"
	case MsgDownlinkNASTransport:
		return "DOWNLINK NAS TRANSPORT"
	case MsgUplinkNASTransport:
		return "UPLINK NAS TRANSPORT"
	case MsgCSServiceNotification:
		return "CS SERVICE NOTIFICATION"
	case MsgDownlinkGenericNASTransport:
		return "DOWNLINK GENERIC NAS TRANSPORT"
	case MsgUplinkGenericNASTransport:
		return "UPLINK GENERIC NAS TRANSPORT"
	default:
		return fmt.Sprintf("unknown message type (%#x)", uint8(t))
	}
}

// String is the message name TS 24.301 table 9.8.2 gives the type.
func (t ESMMessageType) String() string {
	switch t {
	case MsgActivateDefaultEPSBearerContextRequest:
		return "ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST"
	case MsgActivateDefaultEPSBearerContextAccept:
		return "ACTIVATE DEFAULT EPS BEARER CONTEXT ACCEPT"
	case MsgActivateDefaultEPSBearerContextReject:
		return "ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT"
	case MsgActivateDedicatedEPSBearerContextRequest:
		return "ACTIVATE DEDICATED EPS BEARER CONTEXT REQUEST"
	case MsgActivateDedicatedEPSBearerContextAccept:
		return "ACTIVATE DEDICATED EPS BEARER CONTEXT ACCEPT"
	case MsgActivateDedicatedEPSBearerContextReject:
		return "ACTIVATE DEDICATED EPS BEARER CONTEXT REJECT"
	case MsgModifyEPSBearerContextRequest:
		return "MODIFY EPS BEARER CONTEXT REQUEST"
	case MsgModifyEPSBearerContextAccept:
		return "MODIFY EPS BEARER CONTEXT ACCEPT"
	case MsgModifyEPSBearerContextReject:
		return "MODIFY EPS BEARER CONTEXT REJECT"
	case MsgDeactivateEPSBearerContextRequest:
		return "DEACTIVATE EPS BEARER CONTEXT REQUEST"
	case MsgDeactivateEPSBearerContextAccept:
		return "DEACTIVATE EPS BEARER CONTEXT ACCEPT"
	case MsgPDNConnectivityRequest:
		return "PDN CONNECTIVITY REQUEST"
	case MsgPDNConnectivityReject:
		return "PDN CONNECTIVITY REJECT"
	case MsgPDNDisconnectRequest:
		return "PDN DISCONNECT REQUEST"
	case MsgPDNDisconnectReject:
		return "PDN DISCONNECT REJECT"
	case MsgBearerResourceAllocationRequest:
		return "BEARER RESOURCE ALLOCATION REQUEST"
	case MsgBearerResourceAllocationReject:
		return "BEARER RESOURCE ALLOCATION REJECT"
	case MsgBearerResourceModificationRequest:
		return "BEARER RESOURCE MODIFICATION REQUEST"
	case MsgBearerResourceModificationReject:
		return "BEARER RESOURCE MODIFICATION REJECT"
	case MsgESMInformationRequest:
		return "ESM INFORMATION REQUEST"
	case MsgESMInformationResponse:
		return "ESM INFORMATION RESPONSE"
	case MsgNotification:
		return "NOTIFICATION"
	case MsgESMDummyMessage:
		return "ESM DUMMY MESSAGE"
	case MsgESMStatus:
		return "ESM STATUS"
	case MsgRemoteUEReport:
		return "REMOTE UE REPORT"
	case MsgRemoteUEReportResponse:
		return "REMOTE UE REPORT RESPONSE"
	case MsgESMDataTransport:
		return "ESM DATA TRANSPORT"
	default:
		return fmt.Sprintf("unknown message type (%#x)", uint8(t))
	}
}
