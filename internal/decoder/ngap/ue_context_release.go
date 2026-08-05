// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UENGAPIDPair struct {
	AMFUENGAPID int64 `json:"amf_ue_ngap_id"`
	RANUENGAPID int64 `json:"ran_ue_ngap_id"`
}

type UENGAPIDs struct {
	AMFUENGAPID  int64        `json:"amf_ue_ngap_id"`
	UENGAPIDPair UENGAPIDPair `json:"ue_ngap_id_pair"`
}

type PDUSessionResourceItemCxtRelCpl struct {
	PDUSessionID int64
}

type PDUSessionResourceListCxtRelReq struct {
	PDUSessionID int64 `json:"pdu_session_id"`
}

// UENGAPIDs ::= CHOICE { uE-NGAP-ID-pair, aMF-UE-NGAP-ID, ... }. The library
// flattens the CHOICE onto one struct with a Pair discriminator, so only the
// selected alternative is rendered (TS 38.413 §9.3.3.1).
func libUENGAPIDs(ids ngap.UENGAPIDs) UENGAPIDs {
	if ids.Pair {
		return UENGAPIDs{UENGAPIDPair: UENGAPIDPair{
			AMFUENGAPID: int64(ids.AMFUENGAPID),
			RANUENGAPID: int64(ids.RANUENGAPID),
		}}
	}

	return UENGAPIDs{AMFUENGAPID: int64(ids.AMFUENGAPID)}
}

func buildUEContextReleaseRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextReleaseRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Release Request: %v", err)}
	}

	ies := []IE{
		ie(idAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.PDUSessionResourceList != nil {
		sessions := make([]PDUSessionResourceListCxtRelReq, 0, len(m.PDUSessionResourceList))
		for _, item := range m.PDUSessionResourceList {
			sessions = append(sessions, PDUSessionResourceListCxtRelReq{PDUSessionID: int64(item.PDUSessionID)})
		}

		ies = append(ies, ie(idPDUSessionResourceListCxtRelReq, ngap.CriticalityReject, sessions))
	}

	if m.Cause != nil {
		ies = append(ies, ie(idCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextReleaseCommand(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextReleaseCommand(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Release Command: %v", err)}
	}

	ies := []IE{
		ie(idUENGAPIDs, ngap.CriticalityReject, libUENGAPIDs(m.UENGAPIDs)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(idCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildUEContextReleaseComplete(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUEContextReleaseComplete(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Context Release Complete: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(idAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(idRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(idUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	if m.PDUSessionResourceList != nil {
		sessions := make([]PDUSessionResourceItemCxtRelCpl, 0, len(m.PDUSessionResourceList))
		for _, item := range m.PDUSessionResourceList {
			sessions = append(sessions, PDUSessionResourceItemCxtRelCpl{PDUSessionID: int64(item.PDUSessionID)})
		}

		ies = append(ies, ie(idPDUSessionResourceListCxtRelCpl, ngap.CriticalityReject, sessions))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(idCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
