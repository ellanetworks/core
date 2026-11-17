// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
)

const NASTypeUnknown = "Unknown"

var nasMessages *prometheus.CounterVec

func newNASMessages() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_nas_messages_total",
		Help: "Total NAS messages by RAT (EMM/ESM for 4G, 5GMM/5GSM for 5G), direction, and type.",
	}, []string{"rat", "direction", "type"})
}

var gmmTypes = map[fgs.MessageType]string{
	fgs.MsgRegistrationRequest:              "RegistrationRequest",
	fgs.MsgRegistrationAccept:               "RegistrationAccept",
	fgs.MsgRegistrationComplete:             "RegistrationComplete",
	fgs.MsgRegistrationReject:               "RegistrationReject",
	fgs.MsgDeregistrationRequestUEOrig:      "DeregistrationRequestUEOriginating",
	fgs.MsgDeregistrationAcceptUEOrig:       "DeregistrationAcceptUEOriginating",
	fgs.MsgDeregistrationRequestUETerm:      "DeregistrationRequestUETerminated",
	fgs.MsgDeregistrationAcceptUETerm:       "DeregistrationAcceptUETerminated",
	fgs.MsgServiceRequest:                   "ServiceRequest",
	fgs.MsgServiceReject:                    "ServiceReject",
	fgs.MsgServiceAccept:                    "ServiceAccept",
	fgs.MsgControlPlaneServiceRequest:       "ControlPlaneServiceRequest",
	fgs.MsgNetworkSliceSpecificAuthCommand:  "NetworkSliceSpecificAuthenticationCommand",
	fgs.MsgNetworkSliceSpecificAuthComplete: "NetworkSliceSpecificAuthenticationComplete",
	fgs.MsgNetworkSliceSpecificAuthResult:   "NetworkSliceSpecificAuthenticationResult",
	fgs.MsgConfigurationUpdateCommand:       "ConfigurationUpdateCommand",
	fgs.MsgConfigurationUpdateComplete:      "ConfigurationUpdateComplete",
	fgs.MsgAuthenticationRequest:            "AuthenticationRequest",
	fgs.MsgAuthenticationResponse:           "AuthenticationResponse",
	fgs.MsgAuthenticationReject:             "AuthenticationReject",
	fgs.MsgAuthenticationFailure:            "AuthenticationFailure",
	fgs.MsgAuthenticationResult:             "AuthenticationResult",
	fgs.MsgIdentityRequest:                  "IdentityRequest",
	fgs.MsgIdentityResponse:                 "IdentityResponse",
	fgs.MsgSecurityModeCommand:              "SecurityModeCommand",
	fgs.MsgSecurityModeComplete:             "SecurityModeComplete",
	fgs.MsgSecurityModeReject:               "SecurityModeReject",
	fgs.MsgGMMStatus:                        "5GMMStatus",
	fgs.MsgNotification:                     "Notification",
	fgs.MsgNotificationResponse:             "NotificationResponse",
	fgs.MsgULNASTransport:                   "ULNASTransport",
	fgs.MsgDLNASTransport:                   "DLNASTransport",
	fgs.MsgRelayKeyRequest:                  "RelayKeyRequest",
	fgs.MsgRelayKeyAccept:                   "RelayKeyAccept",
	fgs.MsgRelayKeyReject:                   "RelayKeyReject",
	fgs.MsgRelayAuthenticationRequest:       "RelayAuthenticationRequest",
	fgs.MsgRelayAuthenticationResponse:      "RelayAuthenticationResponse",
}

var gsmTypes = map[fgs.GSMMessageType]string{
	fgs.MsgPDUSessionEstablishmentRequest:   "PDUSessionEstablishmentRequest",
	fgs.MsgPDUSessionEstablishmentAccept:    "PDUSessionEstablishmentAccept",
	fgs.MsgPDUSessionEstablishmentReject:    "PDUSessionEstablishmentReject",
	fgs.MsgPDUSessionAuthenticationCommand:  "PDUSessionAuthenticationCommand",
	fgs.MsgPDUSessionAuthenticationComplete: "PDUSessionAuthenticationComplete",
	fgs.MsgPDUSessionAuthenticationResult:   "PDUSessionAuthenticationResult",
	fgs.MsgPDUSessionModificationRequest:    "PDUSessionModificationRequest",
	fgs.MsgPDUSessionModificationReject:     "PDUSessionModificationReject",
	fgs.MsgPDUSessionModificationCommand:    "PDUSessionModificationCommand",
	fgs.MsgPDUSessionModificationComplete:   "PDUSessionModificationComplete",
	fgs.MsgPDUSessionModificationCmdReject:  "PDUSessionModificationCommandReject",
	fgs.MsgPDUSessionReleaseRequest:         "PDUSessionReleaseRequest",
	fgs.MsgPDUSessionReleaseReject:          "PDUSessionReleaseReject",
	fgs.MsgPDUSessionReleaseCommand:         "PDUSessionReleaseCommand",
	fgs.MsgPDUSessionReleaseComplete:        "PDUSessionReleaseComplete",
	fgs.MsgGSMStatus:                        "5GSMStatus",
	fgs.MsgServiceLevelAuthCommand:          "ServiceLevelAuthenticationCommand",
	fgs.MsgServiceLevelAuthComplete:         "ServiceLevelAuthenticationComplete",
	fgs.MsgRemoteUEReport:                   "RemoteUEReport",
	fgs.MsgRemoteUEReportResponse:           "RemoteUEReportResponse",
}

const emmServiceRequest = "ServiceRequest"

var emmTypes = map[eps.MessageType]string{
	eps.MsgAttachRequest:               "AttachRequest",
	eps.MsgAttachAccept:                "AttachAccept",
	eps.MsgAttachComplete:              "AttachComplete",
	eps.MsgAttachReject:                "AttachReject",
	eps.MsgDetachRequest:               "DetachRequest",
	eps.MsgDetachAccept:                "DetachAccept",
	eps.MsgTrackingAreaUpdateRequest:   "TrackingAreaUpdateRequest",
	eps.MsgTrackingAreaUpdateAccept:    "TrackingAreaUpdateAccept",
	eps.MsgTrackingAreaUpdateComplete:  "TrackingAreaUpdateComplete",
	eps.MsgTrackingAreaUpdateReject:    "TrackingAreaUpdateReject",
	eps.MsgExtendedServiceRequest:      "ExtendedServiceRequest",
	eps.MsgControlPlaneServiceRequest:  "ControlPlaneServiceRequest",
	eps.MsgServiceReject:               "ServiceReject",
	eps.MsgServiceAccept:               "ServiceAccept",
	eps.MsgGUTIReallocationCommand:     "GUTIReallocationCommand",
	eps.MsgGUTIReallocationComplete:    "GUTIReallocationComplete",
	eps.MsgAuthenticationRequest:       "AuthenticationRequest",
	eps.MsgAuthenticationResponse:      "AuthenticationResponse",
	eps.MsgAuthenticationReject:        "AuthenticationReject",
	eps.MsgAuthenticationFailure:       "AuthenticationFailure",
	eps.MsgIdentityRequest:             "IdentityRequest",
	eps.MsgIdentityResponse:            "IdentityResponse",
	eps.MsgSecurityModeCommand:         "SecurityModeCommand",
	eps.MsgSecurityModeComplete:        "SecurityModeComplete",
	eps.MsgSecurityModeReject:          "SecurityModeReject",
	eps.MsgEMMStatus:                   "EMMStatus",
	eps.MsgEMMInformation:              "EMMInformation",
	eps.MsgDownlinkNASTransport:        "DownlinkNASTransport",
	eps.MsgUplinkNASTransport:          "UplinkNASTransport",
	eps.MsgCSServiceNotification:       "CSServiceNotification",
	eps.MsgDownlinkGenericNASTransport: "DownlinkGenericNASTransport",
	eps.MsgUplinkGenericNASTransport:   "UplinkGenericNASTransport",
}

var esmTypes = map[eps.ESMMessageType]string{
	eps.MsgActivateDefaultEPSBearerContextRequest:   "ActivateDefaultEPSBearerContextRequest",
	eps.MsgActivateDefaultEPSBearerContextAccept:    "ActivateDefaultEPSBearerContextAccept",
	eps.MsgActivateDefaultEPSBearerContextReject:    "ActivateDefaultEPSBearerContextReject",
	eps.MsgActivateDedicatedEPSBearerContextRequest: "ActivateDedicatedEPSBearerContextRequest",
	eps.MsgActivateDedicatedEPSBearerContextAccept:  "ActivateDedicatedEPSBearerContextAccept",
	eps.MsgActivateDedicatedEPSBearerContextReject:  "ActivateDedicatedEPSBearerContextReject",
	eps.MsgModifyEPSBearerContextRequest:            "ModifyEPSBearerContextRequest",
	eps.MsgModifyEPSBearerContextAccept:             "ModifyEPSBearerContextAccept",
	eps.MsgModifyEPSBearerContextReject:             "ModifyEPSBearerContextReject",
	eps.MsgDeactivateEPSBearerContextRequest:        "DeactivateEPSBearerContextRequest",
	eps.MsgDeactivateEPSBearerContextAccept:         "DeactivateEPSBearerContextAccept",
	eps.MsgPDNConnectivityRequest:                   "PDNConnectivityRequest",
	eps.MsgPDNConnectivityReject:                    "PDNConnectivityReject",
	eps.MsgPDNDisconnectRequest:                     "PDNDisconnectRequest",
	eps.MsgPDNDisconnectReject:                      "PDNDisconnectReject",
	eps.MsgBearerResourceAllocationRequest:          "BearerResourceAllocationRequest",
	eps.MsgBearerResourceAllocationReject:           "BearerResourceAllocationReject",
	eps.MsgBearerResourceModificationRequest:        "BearerResourceModificationRequest",
	eps.MsgBearerResourceModificationReject:         "BearerResourceModificationReject",
	eps.MsgESMInformationRequest:                    "ESMInformationRequest",
	eps.MsgESMInformationResponse:                   "ESMInformationResponse",
	eps.MsgNotification:                             "Notification",
	eps.MsgESMDummyMessage:                          "ESMDummyMessage",
	eps.MsgESMStatus:                                "ESMStatus",
	eps.MsgRemoteUEReport:                           "RemoteUEReport",
	eps.MsgRemoteUEReportResponse:                   "RemoteUEReportResponse",
	eps.MsgESMDataTransport:                         "ESMDataTransport",
}

func NASMessage5G(direction string, msg []byte) {
	countNAS(RAT5G, direction, nasTypes5G(msg))
}

func NASPDU5G(direction string, pdu []byte) {
	countNAS(RAT5G, direction, nasTypes5G(readable5GS(pdu)))
}

func NASMessage4G(direction string, msg []byte) {
	countNAS(RAT4G, direction, nasTypes4G(msg))
}

func NASPDU4G(direction string, pdu []byte) {
	countNAS(RAT4G, direction, nasTypes4G(readableEPS(pdu)))
}

func countNAS(rat, direction string, types []string) {
	if nasMessages == nil {
		return
	}

	for _, t := range types {
		nasMessages.WithLabelValues(rat, direction, t).Inc()
	}
}

const (
	fgsSecurityHeaderLen = 7
	epsSecurityHeaderLen = 6
)

func readable5GS(pdu []byte) []byte {
	if len(pdu) < 2 || fgs.ProtocolDiscriminator(pdu[0]) != fgs.EPD5GMM {
		return pdu
	}

	switch fgs.SecurityHeaderType(pdu[1] & 0x0F) {
	case fgs.SHTPlain:
		return pdu
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedNewContext:
		if len(pdu) <= fgsSecurityHeaderLen {
			return nil
		}

		return pdu[fgsSecurityHeaderLen:]
	default:
		return nil
	}
}

func readableEPS(pdu []byte) []byte {
	if len(pdu) < 1 || eps.ProtocolDiscriminator(pdu[0]&0x0F) != eps.PDEMM {
		return pdu
	}

	switch sht := eps.SecurityHeaderType(pdu[0] >> 4); {
	case sht == eps.SHTPlain || sht >= eps.SHTServiceRequest:
		return pdu
	case sht == eps.SHTIntegrityProtected || sht == eps.SHTIntegrityProtectedNewContext:
		if len(pdu) <= epsSecurityHeaderLen {
			return nil
		}

		return pdu[epsSecurityHeaderLen:]
	default:
		return nil
	}
}

func nasTypes5G(msg []byte) []string {
	if len(msg) < 3 {
		return []string{NASTypeUnknown}
	}

	switch fgs.ProtocolDiscriminator(msg[0]) {
	case fgs.EPD5GSM:
		return []string{gsmType(msg)}
	case fgs.EPD5GMM:
	default:
		return []string{NASTypeUnknown}
	}

	if fgs.SecurityHeaderType(msg[1]&0x0F) != fgs.SHTPlain {
		return []string{NASTypeUnknown}
	}

	mt := fgs.MessageType(msg[2])

	name, ok := gmmTypes[mt]
	if !ok {
		return []string{NASTypeUnknown}
	}

	if sm := n1SMPayload(mt, msg); sm != nil {
		return []string{name, gsmType(sm)}
	}

	return []string{name}
}

func gsmType(msg []byte) string {
	mt, err := fgs.PeekGSMMessageType(msg)
	if err != nil {
		return NASTypeUnknown
	}

	if name, ok := gsmTypes[mt]; ok {
		return name
	}

	return NASTypeUnknown
}

func n1SMPayload(mt fgs.MessageType, msg []byte) []byte {
	if mt != fgs.MsgULNASTransport && mt != fgs.MsgDLNASTransport {
		return nil
	}

	r := nas.NewReader(msg[3:])

	octet, err := r.U8()
	if err != nil || fgs.PayloadContainerType(octet&0x0F) != fgs.PayloadContainerTypeN1SMInfo {
		return nil
	}

	payload, err := r.LVE()
	if err != nil || len(payload) == 0 {
		return nil
	}

	if mt == fgs.MsgDLNASTransport {
		dl, err := fgs.ParseDLNASTransport(msg)
		if err != nil || dl.Cause != nil {
			return nil
		}
	}

	return payload
}

func nasTypes4G(msg []byte) []string {
	if len(msg) < 1 {
		return []string{NASTypeUnknown}
	}

	switch eps.ProtocolDiscriminator(msg[0] & 0x0F) {
	case eps.PDESM:
		return []string{esmType(msg)}
	case eps.PDEMM:
	default:
		return []string{NASTypeUnknown}
	}

	sht := eps.SecurityHeaderType(msg[0] >> 4)
	if sht >= eps.SHTServiceRequest {
		return []string{emmServiceRequest}
	}

	if sht != eps.SHTPlain || len(msg) < 2 {
		return []string{NASTypeUnknown}
	}

	mt := eps.MessageType(msg[1])

	name, ok := emmTypes[mt]
	if !ok {
		return []string{NASTypeUnknown}
	}

	if esm := esmContainer(mt, msg); len(esm) > 0 {
		return []string{name, esmType(esm)}
	}

	return []string{name}
}

func esmType(msg []byte) string {
	mt, err := eps.PeekESMMessageType(msg)
	if err != nil {
		return NASTypeUnknown
	}

	if name, ok := esmTypes[mt]; ok {
		return name
	}

	return NASTypeUnknown
}

func esmContainer(mt eps.MessageType, msg []byte) []byte {
	r := nas.NewReader(msg[2:])

	switch mt {
	case eps.MsgAttachRequest:
		if _, err := r.U8(); err != nil {
			return nil
		}

		if _, err := r.LV(); err != nil {
			return nil
		}

		if _, err := r.LV(); err != nil {
			return nil
		}
	case eps.MsgAttachAccept:
		if _, err := r.Bytes(2); err != nil {
			return nil
		}

		if _, err := r.LV(); err != nil {
			return nil
		}
	case eps.MsgAttachComplete:
	case eps.MsgAttachReject:
		rej, err := eps.ParseAttachReject(msg)
		if err != nil {
			return nil
		}

		return rej.ESMMessageContainer
	default:
		return nil
	}

	esm, err := r.LVE()
	if err != nil {
		return nil
	}

	return esm
}
