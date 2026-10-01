// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

const VendorID uint32 = 10415

const (
	AVPSupportedFeatures      uint32 = 628
	AVPFeatureListID          uint32 = 629
	AVPFeatureList            uint32 = 630
	AVPMSISDN                 uint32 = 701
	AVPSGSNNumber             uint32 = 1489
	AVPMMENumberForMTSMS      uint32 = 1645
	AVPUserIdentifier         uint32 = 3102
	AVPSCAddress              uint32 = 3300
	AVPSMDeliveryOutcome      uint32 = 3316
	AVPAbsentUserDiagnosticSM uint32 = 3322
	AVPSMSMICorrelationID     uint32 = 3324
)

const (
	ResultFirstRegistration                  uint32 = 2001
	ResultSubsequentRegistration             uint32 = 2002
	ResultUnregisteredService                uint32 = 2003
	ResultSuccessServerNameNotStored         uint32 = 2004
	ResultErrorIdentitiesDontMatch           uint32 = 5002
	ResultErrorIdentityNotRegistered         uint32 = 5003
	ResultErrorRoamingNotAllowed             uint32 = 5004
	ResultErrorIdentityAlreadyRegistered     uint32 = 5005
	ResultErrorAuthSchemeNotSupported        uint32 = 5006
	ResultErrorInAssignmentType              uint32 = 5007
	ResultErrorTooMuchData                   uint32 = 5008
	ResultErrorNotSupportedUserData          uint32 = 5009
	ResultErrorFeatureUnsupported            uint32 = 5011
	ResultErrorServingNodeFeatureUnsupported uint32 = 5012
)

const (
	ResultErrorUserUnknown          uint32 = 5001
	ResultErrorAbsentUser           uint32 = 5550
	ResultErrorUserBusyForMTSMS     uint32 = 5551
	ResultErrorFacilityNotSupported uint32 = 5552
	ResultErrorIllegalUser          uint32 = 5553
	ResultErrorIllegalEquipment     uint32 = 5554
	ResultErrorSMDeliveryFailure    uint32 = 5555
	ResultErrorServiceNotSubscribed uint32 = 5556
	ResultErrorServiceBarred        uint32 = 5557
	ResultErrorMWDListFull          uint32 = 5558
)

var experimentalResultNames = map[uint32]string{
	ResultFirstRegistration:                  "DIAMETER_FIRST_REGISTRATION",
	ResultSubsequentRegistration:             "DIAMETER_SUBSEQUENT_REGISTRATION",
	ResultUnregisteredService:                "DIAMETER_UNREGISTERED_SERVICE",
	ResultSuccessServerNameNotStored:         "DIAMETER_SUCCESS_SERVER_NAME_NOT_STORED",
	ResultErrorIdentitiesDontMatch:           "DIAMETER_ERROR_IDENTITIES_DONT_MATCH",
	ResultErrorIdentityNotRegistered:         "DIAMETER_ERROR_IDENTITY_NOT_REGISTERED",
	ResultErrorRoamingNotAllowed:             "DIAMETER_ERROR_ROAMING_NOT_ALLOWED",
	ResultErrorIdentityAlreadyRegistered:     "DIAMETER_ERROR_IDENTITY_ALREADY_REGISTERED",
	ResultErrorAuthSchemeNotSupported:        "DIAMETER_ERROR_AUTH_SCHEME_NOT_SUPPORTED",
	ResultErrorInAssignmentType:              "DIAMETER_ERROR_IN_ASSIGNMENT_TYPE",
	ResultErrorTooMuchData:                   "DIAMETER_ERROR_TOO_MUCH_DATA",
	ResultErrorNotSupportedUserData:          "DIAMETER_ERROR_NOT_SUPPORTED_USER_DATA",
	ResultErrorFeatureUnsupported:            "DIAMETER_ERROR_FEATURE_UNSUPPORTED",
	ResultErrorServingNodeFeatureUnsupported: "DIAMETER_ERROR_SERVING_NODE_FEATURE_UNSUPPORTED",
	ResultErrorUserUnknown:                   "DIAMETER_ERROR_USER_UNKNOWN",
	ResultErrorAbsentUser:                    "DIAMETER_ERROR_ABSENT_USER",
	ResultErrorUserBusyForMTSMS:              "DIAMETER_ERROR_USER_BUSY_FOR_MT_SMS",
	ResultErrorFacilityNotSupported:          "DIAMETER_ERROR_FACILITY_NOT_SUPPORTED",
	ResultErrorIllegalUser:                   "DIAMETER_ERROR_ILLEGAL_USER",
	ResultErrorIllegalEquipment:              "DIAMETER_ERROR_ILLEGAL_EQUIPMENT",
	ResultErrorSMDeliveryFailure:             "DIAMETER_ERROR_SM_DELIVERY_FAILURE",
	ResultErrorServiceNotSubscribed:          "DIAMETER_ERROR_SERVICE_NOT_SUBSCRIBED",
	ResultErrorServiceBarred:                 "DIAMETER_ERROR_SERVICE_BARRED",
	ResultErrorMWDListFull:                   "DIAMETER_ERROR_MWD_LIST_FULL",
}

type AbsentUserDiagnostic uint32

const (
	AbsentUserNoPagingResponseMSC        AbsentUserDiagnostic = 0
	AbsentUserIMSIDetached               AbsentUserDiagnostic = 1
	AbsentUserRoamingRestriction         AbsentUserDiagnostic = 2
	AbsentUserDeregisteredNonGPRS        AbsentUserDiagnostic = 3
	AbsentUserPurgedNonGPRS              AbsentUserDiagnostic = 4
	AbsentUserNoPagingResponseSGSN       AbsentUserDiagnostic = 5
	AbsentUserGPRSDetached               AbsentUserDiagnostic = 6
	AbsentUserDeregisteredGPRS           AbsentUserDiagnostic = 7
	AbsentUserPurgedGPRS                 AbsentUserDiagnostic = 8
	AbsentUserUnidentifiedSubscriberMSC  AbsentUserDiagnostic = 9
	AbsentUserUnidentifiedSubscriberSGSN AbsentUserDiagnostic = 10
	AbsentUserDeregisteredIMS            AbsentUserDiagnostic = 11
	AbsentUserNoResponseIPSMGW           AbsentUserDiagnostic = 12
	AbsentUserTemporarilyUnavailable     AbsentUserDiagnostic = 13
)

var absentUserDiagnosticNames = map[AbsentUserDiagnostic]string{
	AbsentUserNoPagingResponseMSC:        "NO_PAGING_RESPONSE_VIA_THE_MSC",
	AbsentUserIMSIDetached:               "IMSI_DETACHED",
	AbsentUserRoamingRestriction:         "ROAMING_RESTRICTION",
	AbsentUserDeregisteredNonGPRS:        "DEREGISTERED_IN_THE_HLR_FOR_NON_GPRS",
	AbsentUserPurgedNonGPRS:              "MS_PURGED_FOR_NON_GPRS",
	AbsentUserNoPagingResponseSGSN:       "NO_PAGING_RESPONSE_VIA_THE_SGSN",
	AbsentUserGPRSDetached:               "GPRS_DETACHED",
	AbsentUserDeregisteredGPRS:           "DEREGISTERED_IN_THE_HLR_FOR_GPRS",
	AbsentUserPurgedGPRS:                 "MS_PURGED_FOR_GPRS",
	AbsentUserUnidentifiedSubscriberMSC:  "UNIDENTIFIED_SUBSCRIBER_VIA_THE_MSC",
	AbsentUserUnidentifiedSubscriberSGSN: "UNIDENTIFIED_SUBSCRIBER_VIA_THE_SGSN",
	AbsentUserDeregisteredIMS:            "DEREGISTERED_IN_THE_HSS_HLR_FOR_IMS",
	AbsentUserNoResponseIPSMGW:           "NO_RESPONSE_VIA_THE_IP_SM_GW",
	AbsentUserTemporarilyUnavailable:     "THE_MS_IS_TEMPORARILY_UNAVAILABLE",
}

func (d AbsentUserDiagnostic) String() string {
	if name, ok := absentUserDiagnosticNames[d]; ok {
		return name
	}

	return fmt.Sprintf("AbsentUserDiagnostic(%d)", uint32(d))
}

type Envelope struct {
	SessionID        string
	Origin           diameter.Identity
	DestinationHost  string
	DestinationRealm string
}

func (e Envelope) AVPs() []diameter.AVP {
	avps := []diameter.AVP{
		diameter.UTF8String(diameter.AVPSessionID, diameter.AVPFlagMandatory, 0, e.SessionID),
		diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, diameter.AuthSessionStateNoStateMaintained),
		diameter.UTF8String(diameter.AVPOriginHost, diameter.AVPFlagMandatory, 0, e.Origin.OriginHost),
		diameter.UTF8String(diameter.AVPOriginRealm, diameter.AVPFlagMandatory, 0, e.Origin.OriginRealm),
	}

	if e.DestinationHost != "" {
		avps = append(avps, diameter.UTF8String(diameter.AVPDestinationHost, diameter.AVPFlagMandatory, 0, e.DestinationHost))
	}

	return append(avps, diameter.UTF8String(diameter.AVPDestinationRealm, diameter.AVPFlagMandatory, 0, e.DestinationRealm))
}

func ParseEnvelope(m *diameter.Message) Envelope {
	var e Envelope

	for _, f := range []struct {
		code uint32
		dst  *string
	}{
		{diameter.AVPSessionID, &e.SessionID},
		{diameter.AVPOriginHost, &e.Origin.OriginHost},
		{diameter.AVPOriginRealm, &e.Origin.OriginRealm},
		{diameter.AVPDestinationHost, &e.DestinationHost},
		{diameter.AVPDestinationRealm, &e.DestinationRealm},
	} {
		if a, ok := m.Find(f.code, 0); ok {
			*f.dst = a.UTF8String()
		}
	}

	return e
}

func NewAnswer(req *diameter.Message, id diameter.Identity, resultCode uint32) *diameter.Message {
	return withAuthSessionState(diameter.NewAnswer(req, id, resultCode))
}

func NewExperimentalAnswer(req *diameter.Message, id diameter.Identity, resultCode uint32) *diameter.Message {
	return withAuthSessionState(diameter.NewExperimentalAnswer(req, id, VendorID, resultCode))
}

func NewResultAnswer(req *diameter.Message, id diameter.Identity, r Result) *diameter.Message {
	if r.Experimental {
		return withAuthSessionState(diameter.NewExperimentalAnswer(req, id, r.VendorID, r.Code))
	}

	return NewAnswer(req, id, r.Code)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error) *diameter.Message {
	if r, ok := ResultOf(err); ok {
		return NewResultAnswer(req, id, r)
	}

	return withAuthSessionState(diameter.NewErrorAnswer(req, id, err))
}

func InvalidAVP(a diameter.AVP) error {
	return diameter.NewAVPError(diameter.ResultInvalidAVPValue, a)
}

func MissingAVP(code, vendorID uint32) error {
	return diameter.NewAVPError(diameter.ResultMissingAVP, diameter.OctetString(code, diameter.AVPFlagMandatory, vendorID, nil))
}

func withAuthSessionState(ans *diameter.Message) *diameter.Message {
	ans.AVPs = append(ans.AVPs, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0,
		diameter.AuthSessionStateNoStateMaintained))

	return ans
}
