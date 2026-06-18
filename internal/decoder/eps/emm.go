// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type EMMHeader struct {
	MessageType utils.EnumField[uint64] `json:"message_type"`
}

// EMMMessage is a decoded EMM message: its type, plus the salient fields of the
// messages the MME exchanges. Unlisted types decode to the header only.
type EMMMessage struct {
	EMMHeader EMMHeader `json:"emm_header"`
	Error     string    `json:"error,omitempty"`

	AttachRequest             *AttachRequest             `json:"attach_request,omitempty"`
	AttachAccept              *AttachAccept              `json:"attach_accept,omitempty"`
	IdentityRequest           *IdentityRequest           `json:"identity_request,omitempty"`
	IdentityResponse          *IdentityResponse          `json:"identity_response,omitempty"`
	AuthenticationRequest     *AuthenticationRequest     `json:"authentication_request,omitempty"`
	AuthenticationResponse    *AuthenticationResponse    `json:"authentication_response,omitempty"`
	SecurityModeCommand       *SecurityModeCommand       `json:"security_mode_command,omitempty"`
	TrackingAreaUpdateRequest *TrackingAreaUpdateRequest `json:"tracking_area_update_request,omitempty"`
	TrackingAreaUpdateAccept  *TrackingAreaUpdateAccept  `json:"tracking_area_update_accept,omitempty"`
	DetachRequest             *DetachRequest             `json:"detach_request,omitempty"`
	ServiceRequest            *ServiceRequest            `json:"service_request,omitempty"`
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

type AttachRequest struct {
	AttachType     utils.EnumField[uint64] `json:"attach_type"`
	MobileIdentity MobileIdentity          `json:"mobile_identity"`
	ESMContainer   *ESMMessage             `json:"esm_container,omitempty"`
}

type AttachAccept struct {
	AttachResult utils.EnumField[uint64] `json:"attach_result"`
	T3412        uint8                   `json:"t3412"`
	GUTI         *MobileIdentity         `json:"guti,omitempty"`
	EMMCause     *uint8                  `json:"emm_cause,omitempty"`
	ESMContainer *ESMMessage             `json:"esm_container,omitempty"`
}

type IdentityRequest struct {
	IdentityType uint8 `json:"identity_type"`
}

type IdentityResponse struct {
	MobileIdentity string `json:"mobile_identity"` // raw value, hex
}

type AuthenticationRequest struct {
	NASKeySetIdentifier uint8  `json:"nas_key_set_identifier"`
	RAND                string `json:"rand"`
	AUTN                string `json:"autn"`
}

type AuthenticationResponse struct {
	RES string `json:"res"`
}

type SecurityModeCommand struct {
	CipheringAlgorithm utils.EnumField[uint64] `json:"ciphering_algorithm"`
	IntegrityAlgorithm utils.EnumField[uint64] `json:"integrity_algorithm"`
	IMEISVRequested    bool                    `json:"imeisv_requested"`
}

type TrackingAreaUpdateRequest struct {
	UpdateType utils.EnumField[uint64] `json:"update_type"`
	ActiveFlag bool                    `json:"active_flag"`
}

type TrackingAreaUpdateAccept struct {
	UpdateResult utils.EnumField[uint64] `json:"update_result"`
	GUTI         *MobileIdentity         `json:"guti,omitempty"`
	EMMCause     *uint8                  `json:"emm_cause,omitempty"`
}

type DetachRequest struct {
	SwitchOff  bool  `json:"switch_off"`
	DetachType uint8 `json:"detach_type"`
}

type ServiceRequest struct {
	KSI      uint8 `json:"ksi"`
	Sequence uint8 `json:"sequence"`
}

func buildEMMMessage(b []byte) *EMMMessage {
	mt := eps.MessageType(b[1])
	m := &EMMMessage{EMMHeader: EMMHeader{MessageType: emmTypeToEnum(mt)}}

	switch mt {
	case eps.MsgAttachRequest:
		if req, err := eps.ParseAttachRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.AttachRequest = &AttachRequest{
				AttachType:     attachTypeToEnum(req.EPSAttachType),
				MobileIdentity: mobileIdentity(req.EPSMobileIdentity),
				ESMContainer:   decodeESMContainer(req.ESMMessageContainer),
			}
		}
	case eps.MsgAttachAccept:
		if acc, err := eps.ParseAttachAccept(b); err != nil {
			m.Error = err.Error()
		} else {
			a := &AttachAccept{
				AttachResult: attachResultToEnum(acc.EPSAttachResult),
				T3412:        acc.T3412,
				EMMCause:     acc.EMMCause,
				ESMContainer: decodeESMContainer(acc.ESMMessageContainer),
			}
			if acc.GUTI != nil {
				id := mobileIdentity(*acc.GUTI)
				a.GUTI = &id
			}

			m.AttachAccept = a
		}
	case eps.MsgIdentityRequest:
		if req, err := eps.ParseIdentityRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.IdentityRequest = &IdentityRequest{IdentityType: req.IdentityType}
		}
	case eps.MsgIdentityResponse:
		if resp, err := eps.ParseIdentityResponse(b); err != nil {
			m.Error = err.Error()
		} else {
			m.IdentityResponse = &IdentityResponse{MobileIdentity: hex.EncodeToString(resp.MobileIdentity)}
		}
	case eps.MsgAuthenticationRequest:
		if req, err := eps.ParseAuthenticationRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.AuthenticationRequest = &AuthenticationRequest{
				NASKeySetIdentifier: req.NASKeySetIdentifier,
				RAND:                hex.EncodeToString(req.RAND[:]),
				AUTN:                hex.EncodeToString(req.AUTN),
			}
		}
	case eps.MsgAuthenticationResponse:
		if resp, err := eps.ParseAuthenticationResponse(b); err != nil {
			m.Error = err.Error()
		} else {
			m.AuthenticationResponse = &AuthenticationResponse{RES: hex.EncodeToString(resp.RES)}
		}
	case eps.MsgSecurityModeCommand:
		if smc, err := eps.ParseSecurityModeCommand(b); err != nil {
			m.Error = err.Error()
		} else {
			m.SecurityModeCommand = &SecurityModeCommand{
				CipheringAlgorithm: cipheringAlgToEnum(smc.CipheringAlgorithm),
				IntegrityAlgorithm: integrityAlgToEnum(smc.IntegrityAlgorithm),
				IMEISVRequested:    smc.IMEISVRequested,
			}
		}
	case eps.MsgTrackingAreaUpdateRequest:
		if req, err := eps.ParseTrackingAreaUpdateRequest(b); err != nil {
			m.Error = err.Error()
		} else {
			m.TrackingAreaUpdateRequest = &TrackingAreaUpdateRequest{
				UpdateType: updateTypeToEnum(req.EPSUpdateType),
				ActiveFlag: req.ActiveFlag,
			}
		}
	case eps.MsgTrackingAreaUpdateAccept:
		if acc, err := eps.ParseTrackingAreaUpdateAccept(b); err != nil {
			m.Error = err.Error()
		} else {
			a := &TrackingAreaUpdateAccept{
				UpdateResult: updateResultToEnum(acc.EPSUpdateResult),
				EMMCause:     acc.EMMCause,
			}
			if acc.GUTI != nil {
				id := mobileIdentity(*acc.GUTI)
				a.GUTI = &id
			}

			m.TrackingAreaUpdateAccept = a
		}
	case eps.MsgDetachRequest:
		if req, err := eps.ParseDetachRequestUE(b); err != nil {
			m.Error = err.Error()
		} else {
			m.DetachRequest = &DetachRequest{SwitchOff: req.SwitchOff, DetachType: req.TypeOfDetach}
		}
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
	switch id.Type {
	case eps.IdentityGUTI:
		return MobileIdentity{Type: "guti", GUTI: &GUTI{
			MCC: id.MCC, MNC: id.MNC, MMEGroupID: id.MMEGroupID, MMECode: id.MMECode, MTMSI: id.MTMSI,
		}}
	case eps.IdentityIMSI:
		return MobileIdentity{Type: "imsi", IMSI: id.Digits}
	case eps.IdentityIMEI:
		return MobileIdentity{Type: "imei", IMEI: id.Digits}
	default:
		return MobileIdentity{Type: fmt.Sprintf("type-%d", id.Type)}
	}
}
