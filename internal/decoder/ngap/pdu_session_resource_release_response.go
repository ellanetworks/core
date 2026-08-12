// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceReleasedItemRelRes struct {
	PDUSessionID                              int64  `json:"pdu_session_id"`
	PDUSessionResourceReleaseResponseTransfer []byte `json:"pdu_session_resource_release_response_transfer"`
}

// PDU Session Resource Release Response confirms the release per session. The
// response transfer is carried opaquely: TS 38.413 §9.3.4.3 leaves it empty
// unless secondary RAT usage reporting is in use, which this core does not do.
func buildPDUSessionResourceReleaseResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceReleaseResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Release Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.PDUSessionResourceReleased != nil {
		out := make([]PDUSessionResourceReleasedItemRelRes, 0, len(m.PDUSessionResourceReleased))

		for _, item := range m.PDUSessionResourceReleased {
			out = append(out, PDUSessionResourceReleasedItemRelRes{
				PDUSessionID: int64(item.PDUSessionID),
				PDUSessionResourceReleaseResponseTransfer: item.Transfer,
			})
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceReleasedListRelRes, ngap.CriticalityIgnore, out))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore,
			userLocationInformation(*m.UserLocationInformation)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore,
			criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
