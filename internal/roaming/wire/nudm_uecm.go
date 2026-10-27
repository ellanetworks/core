// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import "encoding/json"

const (
	ServiceNudmUECM      = "nudm-uecm"
	NudmUECMAPIRoot      = "/nudm-uecm/v1"
	UECMAmf3GppAccess    = "registrations/amf-3gpp-access"
	UECMSmfRegistrations = "registrations/smf-registrations"
)

type ImsVoPs string

const (
	ImsVoPsHomogeneousSupport      ImsVoPs = "HOMOGENEOUS_SUPPORT"
	ImsVoPsHomogeneousNonSupport   ImsVoPs = "HOMOGENEOUS_NON_SUPPORT"
	ImsVoPsNonHomogeneousOrUnknown ImsVoPs = "NON_HOMOGENEOUS_OR_UNKNOWN"
)

func (v ImsVoPs) Known() bool {
	return oneOf(v, ImsVoPsHomogeneousSupport, ImsVoPsHomogeneousNonSupport, ImsVoPsNonHomogeneousOrUnknown)
}

type DeregistrationReason string

const (
	DeregUEInitialRegistration                 DeregistrationReason = "UE_INITIAL_REGISTRATION"
	DeregUERegistrationAreaChange              DeregistrationReason = "UE_REGISTRATION_AREA_CHANGE"
	DeregSubscriptionWithdrawn                 DeregistrationReason = "SUBSCRIPTION_WITHDRAWN"
	Dereg5GSToEPSMobility                      DeregistrationReason = "5GS_TO_EPS_MOBILITY"
	Dereg5GSToEPSMobilityUEInitialRegistration DeregistrationReason = "5GS_TO_EPS_MOBILITY_UE_INITIAL_REGISTRATION"
	DeregReregistrationRequired                DeregistrationReason = "REREGISTRATION_REQUIRED"
	DeregSMFContextTransferred                 DeregistrationReason = "SMF_CONTEXT_TRANSFERRED"
	DeregDuplicatePDUSession                   DeregistrationReason = "DUPLICATE_PDU_SESSION"
	DeregPDUSessionReactivationRequired        DeregistrationReason = "PDU_SESSION_REACTIVATION_REQUIRED"
	DeregDisasterConditionTerminated           DeregistrationReason = "DISASTER_CONDITION_TERMINATED"
	DeregOperatorDeterminedBarring             DeregistrationReason = "OPERATOR_DETERMINED_BARRING"
	DeregDuplicatePDUSessionEPDG               DeregistrationReason = "DUPLICATE_PDU_SESSION_EPDG"
	Dereg3GPPAccessNotAllowed                  DeregistrationReason = "3GPP_ACCESS_NOT_ALLOWED"
	DeregNon3GPPAccessNotAllowed               DeregistrationReason = "NON_3GPP_ACCESS_NOT_ALLOWED"
)

func (r DeregistrationReason) Known() bool {
	return oneOf(r, DeregUEInitialRegistration, DeregUERegistrationAreaChange, DeregSubscriptionWithdrawn,
		Dereg5GSToEPSMobility, Dereg5GSToEPSMobilityUEInitialRegistration, DeregReregistrationRequired,
		DeregSMFContextTransferred, DeregDuplicatePDUSession, DeregPDUSessionReactivationRequired,
		DeregDisasterConditionTerminated, DeregOperatorDeterminedBarring, DeregDuplicatePDUSessionEPDG,
		Dereg3GPPAccessNotAllowed, DeregNon3GPPAccessNotAllowed)
}

type RegistrationReason string

const RegistrationSMFContextTransferred RegistrationReason = "SMF_CONTEXT_TRANSFERRED"

func (r RegistrationReason) Known() bool {
	return r == RegistrationSMFContextTransferred
}

type Amf3GppAccessRegistration struct {
	AmfInstanceID            string            `json:"amfInstanceId"`
	SupportedFeatures        SupportedFeatures `json:"supportedFeatures,omitempty"`
	PurgeFlag                bool              `json:"purgeFlag,omitempty"`
	Pei                      string            `json:"pei,omitempty"`
	ImsVoPs                  ImsVoPs           `json:"imsVoPs,omitempty"`
	DeregCallbackURI         string            `json:"deregCallbackUri"`
	InitialRegistrationInd   *bool             `json:"initialRegistrationInd,omitempty"`
	EmergencyRegistrationInd *bool             `json:"emergencyRegistrationInd,omitempty"`
	Guami                    Guami             `json:"guami"`
	RatType                  RatType           `json:"ratType"`
	UrrpIndicator            *bool             `json:"urrpIndicator,omitempty"`
	RegistrationTime         DateTime          `json:"registrationTime,omitempty"`
	NoEeSubscriptionInd      *bool             `json:"noEeSubscriptionInd,omitempty"`
	Supi                     string            `json:"supi,omitempty"`
	ReRegistrationRequired   *bool             `json:"reRegistrationRequired,omitempty"`
}

func (r *Amf3GppAccessRegistration) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "guami"); err != nil {
		return err
	}

	type plain Amf3GppAccessRegistration

	return json.Unmarshal(data, (*plain)(r))
}

func (r Amf3GppAccessRegistration) Check(p Policy) error {
	if err := r.Guami.Check(p); err != nil {
		return Nest("/guami", err)
	}

	if err := checkEnum("/ratType", r.RatType, p, false); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	return firstError(
		checkString("/amfInstanceId", r.AmfInstanceID, ValidNfInstanceID, false),
		checkAbsoluteURI("/deregCallbackUri", r.DeregCallbackURI, false),
		checkString("/pei", r.Pei, ValidPei, true),
		checkEnum("/imsVoPs", r.ImsVoPs, p, true),
		checkDateTime("/registrationTime", r.RegistrationTime, true),
		checkString("/supi", r.Supi, ValidSupi, true),
		Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	)
}

type Amf3GppAccessRegistrationModification struct {
	Guami     Guami   `json:"guami"`
	PurgeFlag *bool   `json:"purgeFlag,omitempty"`
	Pei       string  `json:"pei,omitempty"`
	ImsVoPs   ImsVoPs `json:"imsVoPs,omitempty"`
}

func PurgeModification(g Guami) Amf3GppAccessRegistrationModification {
	purge := true

	return Amf3GppAccessRegistrationModification{Guami: g, PurgeFlag: &purge}
}

func (m Amf3GppAccessRegistrationModification) Check(p Policy) error {
	if err := m.Guami.Check(p); err != nil {
		return Nest("/guami", err)
	}

	if p != Strict {
		return nil
	}

	return firstError(checkString("/pei", m.Pei, ValidPei, true), checkEnum("/imsVoPs", m.ImsVoPs, p, true))
}

type SmfRegistration struct {
	SmfInstanceID      string             `json:"smfInstanceId"`
	SupportedFeatures  SupportedFeatures  `json:"supportedFeatures,omitempty"`
	PduSessionID       int                `json:"pduSessionId"`
	SingleNssai        Snssai             `json:"singleNssai"`
	Dnn                string             `json:"dnn,omitempty"`
	EmergencyServices  bool               `json:"emergencyServices,omitempty"`
	PlmnID             PlmnID             `json:"plmnId"`
	DeregCallbackURI   string             `json:"deregCallbackUri,omitempty"`
	RegistrationReason RegistrationReason `json:"registrationReason,omitempty"`
	RegistrationTime   DateTime           `json:"registrationTime,omitempty"`
}

func (r SmfRegistration) Check(p Policy) error {
	if p != Strict {
		return nil
	}

	if err := r.SingleNssai.Check(p); err != nil {
		return Nest("/singleNssai", err)
	}

	if err := r.PlmnID.Check(p); err != nil {
		return Nest("/plmnId", err)
	}

	return firstError(
		checkString("/smfInstanceId", r.SmfInstanceID, ValidNfInstanceID, false),
		checkRange("/pduSessionId", r.PduSessionID, 0, 255, false),
		checkString("/dnn", r.Dnn, ValidDnn, true),
		checkAbsoluteURI("/deregCallbackUri", r.DeregCallbackURI, true),
		checkEnum("/registrationReason", r.RegistrationReason, p, true),
		checkDateTime("/registrationTime", r.RegistrationTime, true),
		Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	)
}

type DeregistrationData struct {
	DeregReason      DeregistrationReason `json:"deregReason"`
	AccessType       AccessType           `json:"accessType,omitempty"`
	PduSessionID     *int                 `json:"pduSessionId,omitempty"`
	NewSmfInstanceID string               `json:"newSmfInstanceId,omitempty"`
	OldGuami         *Guami               `json:"oldGuami,omitempty"`
}

func (d DeregistrationData) Check(p Policy) error {
	if err := checkEnum("/deregReason", d.DeregReason, p, false); err != nil {
		return err
	}

	if err := checkEnum("/accessType", d.AccessType, p, true); err != nil {
		return err
	}

	if d.PduSessionID != nil {
		if err := checkRange("/pduSessionId", *d.PduSessionID, 0, 255, true); err != nil {
			return err
		}
	}

	if err := checkOptional("/oldGuami", d.OldGuami, p); err != nil {
		return err
	}

	if p == Strict {
		return checkString("/newSmfInstanceId", d.NewSmfInstanceID, ValidNfInstanceID, true)
	}

	return nil
}
