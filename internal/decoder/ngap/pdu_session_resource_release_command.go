// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceReleaseCommandTransferDecoded struct {
	Cause Cause `json:"cause"`
}

type PDUSessionResourceToReleaseListRelCmd struct {
	PDUSessionID                             int64                                            `json:"pdu_session_id"`
	PDUSessionResourceReleaseCommandTransfer *PDUSessionResourceReleaseCommandTransferDecoded `json:"pdu_session_resource_release_command_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

// PDU Session Resource Release Command names the sessions the AMF wants
// released, each with its own cause (TS 38.413 §9.2.1.3).
func buildPDUSessionResourceReleaseCommand(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceReleaseCommand(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Release Command: %v", err)}
	}

	ies := []IE{
		ie(idAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(idRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.NASPDU != nil {
		ies = append(ies, ie(idNASPDU, ngap.CriticalityIgnore, libNASPDU(*m.NASPDU)))
	}

	if m.PDUSessionResourceRelease != nil {
		out := make([]PDUSessionResourceToReleaseListRelCmd, 0, len(m.PDUSessionResourceRelease))

		for _, item := range m.PDUSessionResourceRelease {
			entry := PDUSessionResourceToReleaseListRelCmd{PDUSessionID: int64(item.PDUSessionID)}

			transfer, err := ngap.ParsePDUSessionResourceReleaseCommandTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode release command transfer: %v", err)
			} else {
				entry.PDUSessionResourceReleaseCommandTransfer = &PDUSessionResourceReleaseCommandTransferDecoded{
					Cause: cause(transfer.Cause),
				}
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(idPDUSessionResourceToReleaseListRelCmd, ngap.CriticalityReject, out))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
