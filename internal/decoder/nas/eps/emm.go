// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type EMMHeader struct {
	MessageType utils.EnumField `json:"message_type"`
}

// EMMMessage is a decoded EMM message: its type, plus the salient fields of the
// messages the MME exchanges. Unlisted types decode to the header only.
type EMMMessage struct {
	EMMHeader EMMHeader `json:"emm_header"`
	Error     string    `json:"error,omitempty"`

	AttachRequest              *AttachRequest              `json:"attach_request,omitempty"`
	AttachAccept               *AttachAccept               `json:"attach_accept,omitempty"`
	IdentityRequest            *IdentityRequest            `json:"identity_request,omitempty"`
	IdentityResponse           *IdentityResponse           `json:"identity_response,omitempty"`
	AuthenticationRequest      *AuthenticationRequest      `json:"authentication_request,omitempty"`
	AuthenticationResponse     *AuthenticationResponse     `json:"authentication_response,omitempty"`
	SecurityModeCommand        *SecurityModeCommand        `json:"security_mode_command,omitempty"`
	TrackingAreaUpdateRequest  *TrackingAreaUpdateRequest  `json:"tracking_area_update_request,omitempty"`
	TrackingAreaUpdateAccept   *TrackingAreaUpdateAccept   `json:"tracking_area_update_accept,omitempty"`
	DetachRequest              *DetachRequest              `json:"detach_request,omitempty"`
	ServiceRequest             *ServiceRequest             `json:"service_request,omitempty"`
	AttachComplete             *AttachComplete             `json:"attach_complete,omitempty"`
	TrackingAreaUpdateReject   *TrackingAreaUpdateReject   `json:"tracking_area_update_reject,omitempty"`
	SecurityModeComplete       *SecurityModeComplete       `json:"security_mode_complete,omitempty"`
	EMMInformation             *EMMInformation             `json:"emm_information,omitempty"`
	GUTIReallocationCommand    *GUTIReallocationCommand    `json:"guti_reallocation_command,omitempty"`
	GUTIReallocationComplete   *GUTIReallocationComplete   `json:"guti_reallocation_complete,omitempty"`
	AttachReject               *AttachReject               `json:"attach_reject,omitempty"`
	AuthenticationFailure      *AuthenticationFailure      `json:"authentication_failure,omitempty"`
	AuthenticationReject       *AuthenticationReject       `json:"authentication_reject,omitempty"`
	DetachAccept               *DetachAccept               `json:"detach_accept,omitempty"`
	DetachRequestNetwork       *DetachRequestNetwork       `json:"detach_request_network,omitempty"`
	EMMStatus                  *EMMStatus                  `json:"emm_status,omitempty"`
	SecurityModeReject         *SecurityModeReject         `json:"security_mode_reject,omitempty"`
	ServiceAccept              *ServiceAccept              `json:"service_accept,omitempty"`
	ServiceReject              *ServiceReject              `json:"service_reject,omitempty"`
	TrackingAreaUpdateComplete *TrackingAreaUpdateComplete `json:"tracking_area_update_complete,omitempty"`
}

type GUTI struct {
	MCC        string `json:"mcc"`
	MNC        string `json:"mnc"`
	MMEGroupID uint16 `json:"mme_group_id"`
	MMECode    uint8  `json:"mme_code"`
	MTMSI      uint32 `json:"m_tmsi"`
}

// MobileIdentity is a decoded EPS mobile identity (TS 24.301 §9.9.3.12).
type MobileIdentity struct {
	Type string `json:"type"` // imsi / guti / imei
	IMSI string `json:"imsi,omitempty"`
	IMEI string `json:"imei,omitempty"`
	GUTI *GUTI  `json:"guti,omitempty"`
}

type ServiceRequest struct {
	KSI      uint8 `json:"ksi"`
	Sequence uint8 `json:"sequence"`
}

func buildEMMMessage(b []byte) *EMMMessage {
	mt := eps.MessageType(b[1])
	m := &EMMMessage{EMMHeader: EMMHeader{MessageType: emmTypeToEnum(mt)}}

	// A capture carries no direction, and TS 24.301 table 9.8.1 gives DETACH
	// REQUEST one message type for both: decode the uplink variant, the one a
	// UE sends, and fall back to the network variant when that fails, since the
	// two have different layouts and only one of them can parse.
	msg, err := eps.ParseMessage(b, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) && mt == eps.MsgDetachRequest {
		msg, err = eps.ParseMessage(b, nas.DirectionDownlink)
	}

	if err != nil && !nas.SoftOnly(err) {
		m.Error = err.Error()

		return m
	}

	switch msg := msg.(type) {
	case *eps.DetachRequestUE:
		m.DetachRequest = buildDetachRequest(msg)
	case *eps.TrackingAreaUpdateAccept:
		m.TrackingAreaUpdateAccept = buildTrackingAreaUpdateAccept(msg)
	case *eps.TrackingAreaUpdateRequest:
		m.TrackingAreaUpdateRequest = buildTrackingAreaUpdateRequest(msg)
	case *eps.SecurityModeCommand:
		m.SecurityModeCommand = buildSecurityModeCommand(msg)
	case *eps.AuthenticationResponse:
		m.AuthenticationResponse = buildAuthenticationResponse(msg)
	case *eps.AuthenticationRequest:
		m.AuthenticationRequest = buildAuthenticationRequest(msg)
	case *eps.IdentityResponse:
		m.IdentityResponse = buildIdentityResponse(msg)
	case *eps.IdentityRequest:
		m.IdentityRequest = buildIdentityRequest(msg)
	case *eps.AttachAccept:
		m.AttachAccept = buildAttachAccept(msg)
	case *eps.AttachRequest:
		m.AttachRequest = buildAttachRequest(msg)
	case *eps.AttachComplete:
		m.AttachComplete = buildAttachComplete(msg)
	case *eps.TrackingAreaUpdateReject:
		m.TrackingAreaUpdateReject = buildTrackingAreaUpdateReject(msg)
	case *eps.SecurityModeComplete:
		m.SecurityModeComplete = buildSecurityModeComplete(msg)
	case *eps.EMMInformation:
		m.EMMInformation = buildEMMInformation(msg)
	case *eps.GUTIReallocationCommand:
		m.GUTIReallocationCommand = buildGUTIReallocationCommand(msg)
	case *eps.GUTIReallocationComplete:
		m.GUTIReallocationComplete = buildGUTIReallocationComplete(msg)
	case *eps.AttachReject:
		m.AttachReject = buildAttachReject(msg)
	case *eps.AuthenticationFailure:
		m.AuthenticationFailure = buildAuthenticationFailure(msg)
	case *eps.AuthenticationReject:
		m.AuthenticationReject = buildAuthenticationReject(msg)
	case *eps.DetachAccept:
		m.DetachAccept = buildDetachAccept(msg)
	case *eps.DetachRequestNetwork:
		m.DetachRequestNetwork = buildDetachRequestNetwork(msg)
	case *eps.EMMStatus:
		m.EMMStatus = buildEMMStatus(msg)
	case *eps.SecurityModeReject:
		m.SecurityModeReject = buildSecurityModeReject(msg)
	case *eps.ServiceAccept:
		m.ServiceAccept = buildServiceAccept(msg)
	case *eps.ServiceReject:
		m.ServiceReject = buildServiceReject(msg)
	case *eps.TrackingAreaUpdateComplete:
		m.TrackingAreaUpdateComplete = buildTrackingAreaUpdateComplete(msg)
	}

	return m
}

func decodeServiceRequest(msg *NASMessage, raw []byte) *NASMessage {
	req, err := eps.ParseServiceRequest(raw)
	if err != nil {
		msg.Error = err.Error()
		return msg
	}

	msg.EMMMessage = &EMMMessage{
		EMMHeader:      EMMHeader{MessageType: utils.MakeEnum(uint64(eps.SHTServiceRequest), "Service Request", false)},
		ServiceRequest: &ServiceRequest{KSI: req.KSI, Sequence: req.SeqShort},
	}

	return msg
}

func mobileIdentity(id eps.EPSMobileIdentity) MobileIdentity {
	switch {
	case id.GUTI != nil:
		return MobileIdentity{Type: "guti", GUTI: &GUTI{
			MCC: id.GUTI.PLMN.MCC, MNC: id.GUTI.PLMN.MNC,
			MMEGroupID: id.GUTI.MMEGroupID, MMECode: id.GUTI.MMECode,
			MTMSI: binary.BigEndian.Uint32(id.GUTI.TMSI[:]),
		}}
	case id.IMSI != nil:
		return MobileIdentity{Type: "imsi", IMSI: string(*id.IMSI)}
	case id.IMEI != nil:
		return MobileIdentity{Type: "imei", IMEI: string(*id.IMEI)}
	default:
		return MobileIdentity{Type: fmt.Sprintf("type-%d", id.Type())}
	}
}

// emmCauseValue narrows a typed EMM cause pointer to the raw value the decoder's
// JSON shape carries.
func emmCauseValue(c *eps.EMMCause) *uint8 {
	if c == nil {
		return nil
	}

	v := uint8(*c)

	return &v
}

// timerOctet narrows a GPRS timer to the raw octet the decoder's JSON shape
// carries.
func timerOctet(t nas.GPRSTimer2) uint8 {
	raw, err := t.MarshalBinary()
	if err != nil || len(raw) == 0 {
		return 0
	}

	return raw[0]
}
