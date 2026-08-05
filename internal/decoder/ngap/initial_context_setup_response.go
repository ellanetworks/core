// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceFailedToSetupCxtRes struct {
	PDUSessionID                                int64                                               `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer *PDUSessionResourceSetupUnsuccessfulTransferDecoded `json:"pdu_session_resource_setup_unsuccessful_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

type PDUSessionResourceSetupCxtRes struct {
	PDUSessionID                            int64                                           `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer *PDUSessionResourceSetupResponseTransferDecoded `json:"pdu_session_resource_setup_response_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

// libUnsuccessfulTransfer decodes the per-session failure transfer the three
// setup responses share (TS 38.413 §9.3.4.16).
func libUnsuccessfulTransfer(raw ngap.TransferContainer) (*PDUSessionResourceSetupUnsuccessfulTransferDecoded, error) {
	t, err := ngap.ParsePDUSessionResourceSetupUnsuccessfulTransfer(raw)
	if err != nil {
		return nil, err
	}

	return &PDUSessionResourceSetupUnsuccessfulTransferDecoded{Cause: cause(t.Cause)}, nil
}

// Initial Context Setup Response reports which sessions the NG-RAN node set up
// alongside the UE context (TS 38.413 §9.2.2.2).
func buildInitialContextSetupResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParseInitialContextSetupResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(idAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(idRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.PDUSessionResourceSetup != nil {
		out := make([]PDUSessionResourceSetupCxtRes, 0, len(m.PDUSessionResourceSetup))

		for _, item := range m.PDUSessionResourceSetup {
			entry := PDUSessionResourceSetupCxtRes{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := ngap.ParsePDUSessionResourceSetupResponseTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode response transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupResponseTransfer = libSetupResponseTransfer(transfer)
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(idPDUSessionResourceSetupListCxtRes, ngap.CriticalityIgnore, out))
	}

	if m.PDUSessionResourceFailed != nil {
		out := make([]PDUSessionResourceFailedToSetupCxtRes, 0, len(m.PDUSessionResourceFailed))

		for _, item := range m.PDUSessionResourceFailed {
			entry := PDUSessionResourceFailedToSetupCxtRes{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := libUnsuccessfulTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode unsuccessful transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupUnsuccessfulTransfer = transfer
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(idPDUSessionResourceFailedToSetupListCxtRes, ngap.CriticalityIgnore, out))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(idCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
