// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"

	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// NetworkName is a network name the AMF pushes to the UE (TS 24.008 §10.5.3.5a).
type NetworkName struct {
	Name   string `json:"name"`
	Coding string `json:"coding"`
	AddCI  bool   `json:"add_country_initials,omitempty"`
}

// ConfigurationUpdateIndication says what the AMF wants back from the UE
// (TS 24.501 §9.11.3.18).
type ConfigurationUpdateIndication struct {
	Acknowledgement      bool `json:"acknowledgement,omitempty"`
	RegistrationRequired bool `json:"registration_requested,omitempty"`
}

type ConfigurationUpdateCommand struct {
	ConfigurationUpdateIndication *ConfigurationUpdateIndication `json:"configuration_update_indication,omitempty"`
	GUTI                          *MobileIdentity                `json:"guti,omitempty"`
	FullNameForNetwork            *NetworkName                   `json:"full_name_for_network,omitempty"`
	ShortNameForNetwork           *NetworkName                   `json:"short_name_for_network,omitempty"`
	LocalTimeZone                 *string                        `json:"local_time_zone,omitempty"`
	UniversalTime                 *string                        `json:"universal_time,omitempty"`
	DaylightSavingTime            *uint8                         `json:"daylight_saving_time,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func networkName(n *naslib.NetworkName) *NetworkName {
	if n == nil {
		return nil
	}

	return &NetworkName{Name: n.Name, Coding: n.Coding.String(), AddCI: n.AddCI}
}

func timeZone(z *naslib.TimeZone) *string {
	if z == nil {
		return nil
	}

	s := z.String()

	return &s
}

func buildConfigurationUpdateCommand(msg *fgs.ConfigurationUpdateCommand) *ConfigurationUpdateCommand {
	out := &ConfigurationUpdateCommand{
		FullNameForNetwork:  networkName(msg.FullNameForNetwork),
		ShortNameForNetwork: networkName(msg.ShortNameForNetwork),
		LocalTimeZone:       timeZone(msg.LocalTimeZone),
	}

	if ind := msg.ConfigurationUpdateIndication; ind != nil {
		out.ConfigurationUpdateIndication = &ConfigurationUpdateIndication{
			Acknowledgement:      ind.ACK,
			RegistrationRequired: ind.RED,
		}
	}

	if msg.GUTI != nil {
		id := buildMobileIdentity(*msg.GUTI)
		out.GUTI = &id
	}

	if msg.UniversalTime != nil {
		if t, ok := msg.UniversalTime.Time(); ok {
			s := t.Format("2006-01-02T15:04:05Z07:00")
			out.UniversalTime = &s
		}
	}

	if msg.DaylightSavingTime != nil {
		v := uint8(*msg.DaylightSavingTime)
		out.DaylightSavingTime = &v
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type ConfigurationUpdateComplete struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildConfigurationUpdateComplete(msg *fgs.ConfigurationUpdateComplete) *ConfigurationUpdateComplete {
	return &ConfigurationUpdateComplete{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

// DeregistrationRequestUEOriginating is the UE telling the network it is leaving
// (TS 24.501 §8.2.12).
type DeregistrationRequestUEOriginating struct {
	AccessType             utils.EnumField `json:"access_type"`
	ReRegistrationRequired bool            `json:"re_registration_required,omitempty"`
	SwitchOff              bool            `json:"switch_off,omitempty"`
	NgKSI                  uint8           `json:"ng_ksi"`
	MobileIdentity         MobileIdentity  `json:"mobile_identity"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDeregistrationRequestUEOriginating(msg *fgs.DeregistrationRequestUEOriginating) *DeregistrationRequestUEOriginating {
	out := &DeregistrationRequestUEOriginating{
		AccessType:             accessTypeToEnum(msg.AccessType),
		ReRegistrationRequired: msg.ReRegistrationRequired,
		SwitchOff:              msg.SwitchOff,
		NgKSI:                  msg.NgKSI.HalfOctet(),
		MobileIdentity:         buildMobileIdentity(msg.MobileIdentity),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// DeregistrationRequestUETerminated is the network detaching the UE
// (TS 24.501 §8.2.14).
type DeregistrationRequestUETerminated struct {
	AccessType             utils.EnumField  `json:"access_type"`
	ReRegistrationRequired bool             `json:"re_registration_required,omitempty"`
	SwitchOff              bool             `json:"switch_off,omitempty"`
	Cause                  *utils.EnumField `json:"cause_5gmm,omitempty"`
	T3346                  *uint8           `json:"t3346,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDeregistrationRequestUETerminated(msg *fgs.DeregistrationRequestUETerminated) *DeregistrationRequestUETerminated {
	out := &DeregistrationRequestUETerminated{
		AccessType:             accessTypeToEnum(msg.AccessType),
		ReRegistrationRequired: msg.ReRegistrationRequired,
		SwitchOff:              msg.SwitchOff,
		T3346:                  timerOctetPtr(msg.T3346),
	}

	if msg.Cause != nil {
		c := cause5GMMToEnum(*msg.Cause)
		out.Cause = &c
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type DeregistrationAccept struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDeregistrationAcceptUEOriginating(msg *fgs.DeregistrationAcceptUEOriginating) *DeregistrationAccept {
	return &DeregistrationAccept{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

func buildDeregistrationAcceptUETerminated(msg *fgs.DeregistrationAcceptUETerminated) *DeregistrationAccept {
	return &DeregistrationAccept{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

// GMMCauseOnly is the shape of a 5GMM message whose only content is a cause.
type GMMCauseOnly struct {
	Cause utils.EnumField `json:"cause_5gmm"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func gmmCauseOnly(c fgs.GMMCause, unrecognized []naslib.RawIE) *GMMCauseOnly {
	return &GMMCauseOnly{Cause: cause5GMMToEnum(c), UnrecognizedIEs: utils.RawIEs(unrecognized)}
}

type NotificationResponse struct {
	PDUSessionStatus []PDUSessionStatusPDU `json:"pdu_session_status,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildNotificationResponse(msg *fgs.NotificationResponse) *NotificationResponse {
	out := &NotificationResponse{PDUSessionStatus: decodePDUSessionStatus(msg.PDUSessionStatus)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// PDUSessionAuthenticationComplete carries the UE's final EAP message for a
// session-level authentication (TS 24.501 §8.3.5).
type PDUSessionAuthenticationComplete struct {
	EAP         string                              `json:"eap,omitempty"`
	ExtendedPCO *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionAuthenticationComplete(msg *fgs.PDUSessionAuthenticationComplete) *PDUSessionAuthenticationComplete {
	out := &PDUSessionAuthenticationComplete{ExtendedPCO: nasie.ExtendedPCO(msg.ExtendedPCO)}

	if len(msg.EAP) > 0 {
		out.EAP = hex.EncodeToString(msg.EAP)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func accessTypeToEnum(a fgs.AccessType) utils.EnumField {
	return utils.NamedEnum(uint8(a), a.String())
}
