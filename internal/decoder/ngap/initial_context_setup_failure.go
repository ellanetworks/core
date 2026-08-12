// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceFailedToSetupCxtFail struct {
	PDUSessionID                                int64                                               `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer *PDUSessionResourceSetupUnsuccessfulTransferDecoded `json:"pdu_session_resource_setup_unsuccessful_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

// Initial Context Setup Failure reports that the NG-RAN node could not set up
// the UE context at all (TS 38.413 §9.2.2.3).
func buildInitialContextSetupFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseInitialContextSetupFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.PDUSessionResourceFailed != nil {
		out := make([]PDUSessionResourceFailedToSetupCxtFail, 0, len(m.PDUSessionResourceFailed))

		for _, item := range m.PDUSessionResourceFailed {
			entry := PDUSessionResourceFailedToSetupCxtFail{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := libUnsuccessfulTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode unsuccessful transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupUnsuccessfulTransfer = transfer
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToSetupListCxtFail, ngap.CriticalityIgnore, out))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
