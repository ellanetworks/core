// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
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

func emmTypeToEnum(mt eps.MessageType) utils.EnumField[uint64] {
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

func esmTypeToEnum(mt eps.ESMMessageType) utils.EnumField[uint64] {
	name, ok := esmMessageNames[mt]

	return utils.MakeEnum(uint64(mt), name, !ok)
}

func attachTypeToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{1: "EPS attach", 2: "combined EPS/IMSI attach", 6: "EPS emergency attach"}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

func attachResultToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{1: "EPS only", 2: "combined EPS/IMSI"}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

func updateTypeToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{
		0: "TA updating",
		1: "combined TA/LA updating",
		2: "combined TA/LA updating with IMSI attach",
		3: "periodic updating",
	}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

func updateResultToEnum(v uint8) utils.EnumField[uint64] {
	names := map[uint8]string{0: "TA updated", 1: "combined TA/LA updated"}

	name, ok := names[v]

	return utils.MakeEnum(uint64(v), name, !ok)
}

// 4G ciphering/integrity algorithms (TS 33.401 §5): EEA/EIA 0-3.
func cipheringAlgToEnum(v uint8) utils.EnumField[uint64] {
	return algToEnum(v, "EEA")
}

func integrityAlgToEnum(v uint8) utils.EnumField[uint64] {
	return algToEnum(v, "EIA")
}

func algToEnum(v uint8, prefix string) utils.EnumField[uint64] {
	if v <= 7 {
		return utils.MakeEnum(uint64(v), prefix+string(rune('0'+v)), false)
	}

	return utils.MakeEnum(uint64(v), "", true)
}
