// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

var emmMessageNames = map[eps.MessageType]string{
	eps.MsgAttachRequest:              "Attach Request",
	eps.MsgAttachAccept:               "Attach Accept",
	eps.MsgAttachComplete:             "Attach Complete",
	eps.MsgAttachReject:               "Attach Reject",
	eps.MsgDetachRequest:              "Detach Request",
	eps.MsgDetachAccept:               "Detach Accept",
	eps.MsgTrackingAreaUpdateRequest:  "Tracking Area Update Request",
	eps.MsgTrackingAreaUpdateAccept:   "Tracking Area Update Accept",
	eps.MsgTrackingAreaUpdateComplete: "Tracking Area Update Complete",
	eps.MsgTrackingAreaUpdateReject:   "Tracking Area Update Reject",
	eps.MsgServiceReject:              "Service Reject",
	eps.MsgAuthenticationRequest:      "Authentication Request",
	eps.MsgAuthenticationResponse:     "Authentication Response",
	eps.MsgAuthenticationReject:       "Authentication Reject",
	eps.MsgAuthenticationFailure:      "Authentication Failure",
	eps.MsgIdentityRequest:            "Identity Request",
	eps.MsgIdentityResponse:           "Identity Response",
	eps.MsgSecurityModeCommand:        "Security Mode Command",
	eps.MsgSecurityModeComplete:       "Security Mode Complete",
	eps.MsgSecurityModeReject:         "Security Mode Reject",
	eps.MsgEMMStatus:                  "EMM Status",
	eps.MsgEMMInformation:             "EMM Information",
}

func emmTypeToEnum(mt eps.MessageType) utils.EnumField {
	name, ok := emmMessageNames[mt]

	return utils.MakeEnum(uint64(mt), name, !ok)
}

var esmMessageNames = map[eps.ESMMessageType]string{
	eps.MsgActivateDefaultEPSBearerContextRequest: "Activate Default EPS Bearer Context Request",
	eps.MsgActivateDefaultEPSBearerContextAccept:  "Activate Default EPS Bearer Context Accept",
	eps.MsgActivateDefaultEPSBearerContextReject:  "Activate Default EPS Bearer Context Reject",
	eps.MsgPDNConnectivityRequest:                 "PDN Connectivity Request",
	eps.MsgPDNConnectivityReject:                  "PDN Connectivity Reject",
	eps.MsgESMInformationRequest:                  "ESM Information Request",
	eps.MsgESMInformationResponse:                 "ESM Information Response",
	eps.MsgESMStatus:                              "ESM Status",
}

func esmTypeToEnum(mt eps.ESMMessageType) utils.EnumField {
	name, ok := esmMessageNames[mt]

	return utils.MakeEnum(uint64(mt), name, !ok)
}

func attachTypeToEnum(v eps.AttachType) utils.EnumField {
	return typedEnum(uint8(v), v.String())
}

func attachResultToEnum(v eps.AttachResult) utils.EnumField {
	return typedEnum(uint8(v), v.String())
}

func updateTypeToEnum(v eps.EPSUpdateType) utils.EnumField {
	return typedEnum(uint8(v), v.String())
}

func updateResultToEnum(v eps.EPSUpdateResult) utils.EnumField {
	return typedEnum(uint8(v), v.String())
}

// typedEnum renders a library enumeration, marking a value the library does not
// name as unknown.
func typedEnum(v uint8, name string) utils.EnumField {
	if strings.HasPrefix(name, "unknown") {
		return utils.MakeEnum(uint64(v), "", true)
	}

	return utils.MakeEnum(uint64(v), name, false)
}

// 4G ciphering/integrity algorithms (TS 33.401 §5): EEA/EIA 0-3.
func cipheringAlgToEnum(v uint8) utils.EnumField {
	return algToEnum(v, "EEA")
}

func integrityAlgToEnum(v uint8) utils.EnumField {
	return algToEnum(v, "EIA")
}

func algToEnum(v uint8, prefix string) utils.EnumField {
	if v <= 7 {
		return utils.MakeEnum(uint64(v), prefix+string(rune('0'+v)), false)
	}

	return utils.MakeEnum(uint64(v), "", true)
}
