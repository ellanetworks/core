// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/binary"
	"encoding/hex"
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
	AttachType     utils.EnumField `json:"attach_type"`
	MobileIdentity MobileIdentity  `json:"mobile_identity"`
	ESMContainer   *ESMMessage     `json:"esm_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type AttachAccept struct {
	AttachResult utils.EnumField `json:"attach_result"`
	T3412        uint8           `json:"t3412"`
	GUTI         *MobileIdentity `json:"guti,omitempty"`
	EMMCause     *uint8          `json:"emm_cause,omitempty"`
	ESMContainer *ESMMessage     `json:"esm_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type IdentityRequest struct {
	IdentityType uint8 `json:"identity_type"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type IdentityResponse struct {
	MobileIdentity string `json:"mobile_identity"` // raw value, hex

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type AuthenticationRequest struct {
	NASKeySetIdentifier uint8  `json:"nas_key_set_identifier"`
	RAND                string `json:"rand"`
	AUTN                string `json:"autn"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type AuthenticationResponse struct {
	RES string `json:"res"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type SecurityModeCommand struct {
	CipheringAlgorithm utils.EnumField `json:"ciphering_algorithm"`
	IntegrityAlgorithm utils.EnumField `json:"integrity_algorithm"`
	IMEISVRequested    bool            `json:"imeisv_requested"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type TrackingAreaUpdateRequest struct {
	UpdateType utils.EnumField `json:"update_type"`
	ActiveFlag bool            `json:"active_flag"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type TrackingAreaUpdateAccept struct {
	UpdateResult utils.EnumField `json:"update_result"`
	GUTI         *MobileIdentity `json:"guti,omitempty"`
	EMMCause     *uint8          `json:"emm_cause,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

type DetachRequest struct {
	SwitchOff  bool  `json:"switch_off"`
	DetachType uint8 `json:"detach_type"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
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
	// UE sends.
	msg, err := eps.ParseMessage(b, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) {
		m.Error = err.Error()

		return m
	}

	switch msg := msg.(type) {
	case *eps.AttachRequest:
		m.AttachRequest = &AttachRequest{
			AttachType:     attachTypeToEnum(msg.EPSAttachType),
			MobileIdentity: mobileIdentity(msg.EPSMobileIdentity),
			ESMContainer:   decodeESMContainer(msg.ESMMessageContainer),
		}

		m.AttachRequest.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.AttachAccept:
		a := &AttachAccept{
			AttachResult: attachResultToEnum(msg.EPSAttachResult),
			T3412:        timerOctet(msg.T3412),
			EMMCause:     emmCauseValue(msg.Cause),
			ESMContainer: decodeESMContainer(msg.ESMMessageContainer),
		}
		if msg.GUTI != nil {
			id := mobileIdentity(*msg.GUTI)
			a.GUTI = &id
		}

		m.AttachAccept = a

		m.AttachAccept.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.IdentityRequest:
		m.IdentityRequest = &IdentityRequest{IdentityType: uint8(msg.IdentityType)}

		m.IdentityRequest.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.IdentityResponse:
		m.IdentityResponse = &IdentityResponse{MobileIdentity: msg.MobileIdentity.String()}

		m.IdentityResponse.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.AuthenticationRequest:
		m.AuthenticationRequest = &AuthenticationRequest{
			NASKeySetIdentifier: msg.NASKeySetIdentifier.HalfOctet(),
			RAND:                hex.EncodeToString(msg.RAND[:]),
			AUTN:                hex.EncodeToString(msg.AUTN[:]),
		}

		m.AuthenticationRequest.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.AuthenticationResponse:
		m.AuthenticationResponse = &AuthenticationResponse{RES: hex.EncodeToString(msg.RES)}

		m.AuthenticationResponse.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.SecurityModeCommand:
		m.SecurityModeCommand = &SecurityModeCommand{
			CipheringAlgorithm: cipheringAlgToEnum(uint8(msg.CipheringAlgorithm)),
			IntegrityAlgorithm: integrityAlgToEnum(uint8(msg.IntegrityAlgorithm)),
			IMEISVRequested:    msg.IMEISVRequested != nil && msg.IMEISVRequested.Requested(),
		}

		m.SecurityModeCommand.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.TrackingAreaUpdateRequest:
		m.TrackingAreaUpdateRequest = &TrackingAreaUpdateRequest{
			UpdateType: updateTypeToEnum(msg.EPSUpdateType),
			ActiveFlag: msg.ActiveFlag,
		}

		m.TrackingAreaUpdateRequest.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.TrackingAreaUpdateAccept:
		a := &TrackingAreaUpdateAccept{
			UpdateResult: updateResultToEnum(msg.EPSUpdateResult),
			EMMCause:     emmCauseValue(msg.Cause),
		}
		if msg.GUTI != nil {
			id := mobileIdentity(*msg.GUTI)
			a.GUTI = &id
		}

		m.TrackingAreaUpdateAccept = a

		m.TrackingAreaUpdateAccept.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
	case *eps.DetachRequestUE:
		m.DetachRequest = &DetachRequest{SwitchOff: msg.SwitchOff, DetachType: uint8(msg.TypeOfDetach)}

		m.DetachRequest.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)
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
