// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
)

type PDUSessionCause struct {
	PDUSessionID uint8           `json:"pdu_session_id"`
	Cause        utils.EnumField `json:"cause"`
}

type PDUSessionReactivateResultPDU struct {
	PDUSessionID int `json:"pdu_session_id"`
	// EstablishmentFailed is the set bit of TS 24.501 §9.11.3.42: user-plane
	// resources the UE asked for, or the network allowed, were not established.
	EstablishmentFailed bool `json:"establishment_failed"`
}

type ServiceAccept struct {
	PDUSessionStatus                       []PDUSessionStatusPDU           `json:"pdu_session_status,omitempty"`
	PDUSessionReactivationResult           []PDUSessionReactivateResultPDU `json:"pdu_session_reactivation_result,omitempty"`
	PDUSessionReactivationResultErrorCause []PDUSessionCause               `json:"pdu_session_reactivation_result_error_cause,omitempty"`
	EAPMessage                             *RawOctets                      `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildServiceAccept(msg *fgs.ServiceAccept) *ServiceAccept {
	out := &ServiceAccept{
		EAPMessage: rawOctets(msg.EAP),
	}

	if msg.PDUSessionStatus != nil {
		out.PDUSessionStatus = decodePDUSessionStatus(msg.PDUSessionStatus)
	}

	out.PDUSessionReactivationResult = decodePDUSessionReactivationResult(msg.PDUSessionReactivationResult)

	if msg.ReactivationResultErrorCause != nil {
		pduSessionIDs, causes := bufToPDUSessionReactivationResultErrorCause(msg.ReactivationResultErrorCause)
		if len(pduSessionIDs) != len(causes) {
			logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause: invalid length")
		} else {
			var pduSessionCauses []PDUSessionCause

			for i := range pduSessionIDs {
				pduSessionCauses = append(pduSessionCauses, PDUSessionCause{
					PDUSessionID: pduSessionIDs[i],
					Cause:        cause5GMMToEnum(fgs.GMMCause(causes[i])),
				})
			}

			out.PDUSessionReactivationResultErrorCause = pduSessionCauses
		}
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func bufToPDUSessionReactivationResultErrorCause(errs fgs.ReactivationResultErrorCause) (errPduSessionID, errCause []uint8) {
	errPduSessionID = make([]uint8, 0, len(errs))
	errCause = make([]uint8, 0, len(errs))

	for _, e := range errs {
		errPduSessionID = append(errPduSessionID, uint8(e.PDUSessionID))
		errCause = append(errCause, uint8(e.Cause))
	}

	return errPduSessionID, errCause
}
