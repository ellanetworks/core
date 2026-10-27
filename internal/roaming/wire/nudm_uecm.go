// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

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

type Amf3GppAccessRegistration struct {
	AmfInstanceID          string            `json:"amfInstanceId"`
	SupportedFeatures      SupportedFeatures `json:"supportedFeatures,omitempty"`
	Pei                    string            `json:"pei,omitempty"`
	ImsVoPs                ImsVoPs           `json:"imsVoPs,omitempty"`
	DeregCallbackURI       string            `json:"deregCallbackUri"`
	InitialRegistrationInd *bool             `json:"initialRegistrationInd,omitempty"`
	Guami                  *Guami            `json:"guami"`
	RatType                RatType           `json:"ratType"`
}

func (r Amf3GppAccessRegistration) Check(p Policy) error {
	if err := firstError(
		present("/amfInstanceId", r.AmfInstanceID != ""),
		present("/deregCallbackUri", r.DeregCallbackURI != ""),
		present("/guami", r.Guami != nil),
		present("/ratType", r.RatType != ""),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkString("/amfInstanceId", r.AmfInstanceID, ValidNfInstanceID, false),
		checkAbsoluteURI("/deregCallbackUri", r.DeregCallbackURI, false),
		CheckRequired("/guami", r.Guami, p),
		CheckEnum("/ratType", r.RatType, p, false),
		checkString("/pei", r.Pei, ValidPei, true),
		CheckEnum("/imsVoPs", r.ImsVoPs, p, true),
		Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	)
}

type Amf3GppAccessRegistrationModification struct {
	Guami     *Guami `json:"guami"`
	PurgeFlag *bool  `json:"purgeFlag,omitempty"`
}

func PurgeModification(g Guami) Amf3GppAccessRegistrationModification {
	purge := true

	return Amf3GppAccessRegistrationModification{Guami: &g, PurgeFlag: &purge}
}

func (m Amf3GppAccessRegistrationModification) Check(p Policy) error {
	if err := present("/guami", m.Guami != nil); err != nil || p != Strict {
		return err
	}

	return CheckRequired("/guami", m.Guami, p)
}

type SmfRegistration struct {
	SmfInstanceID     string            `json:"smfInstanceId"`
	SupportedFeatures SupportedFeatures `json:"supportedFeatures,omitempty"`
	PduSessionID      *int              `json:"pduSessionId"`
	SingleNssai       *Snssai           `json:"singleNssai"`
	Dnn               string            `json:"dnn,omitempty"`
	PlmnID            *PlmnID           `json:"plmnId"`
}

func (r SmfRegistration) Check(p Policy) error {
	if err := firstError(
		present("/smfInstanceId", r.SmfInstanceID != ""),
		present("/pduSessionId", r.PduSessionID != nil),
		present("/singleNssai", r.SingleNssai != nil),
		present("/plmnId", r.PlmnID != nil),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkString("/smfInstanceId", r.SmfInstanceID, ValidNfInstanceID, false),
		checkRequiredRange("/pduSessionId", r.PduSessionID, 0, 255),
		CheckRequired("/singleNssai", r.SingleNssai, p),
		checkString("/dnn", r.Dnn, ValidDnn, true),
		CheckRequired("/plmnId", r.PlmnID, p),
		Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	)
}

type DeregistrationData struct {
	DeregReason DeregistrationReason `json:"deregReason"`
	AccessType  AccessType           `json:"accessType,omitempty"`
}

func (d DeregistrationData) Check(p Policy) error {
	return firstError(
		CheckEnum("/deregReason", d.DeregReason, p, false),
		CheckEnum("/accessType", d.AccessType, p, true),
	)
}
