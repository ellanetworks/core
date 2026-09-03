// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type PDUSessionResourceSetupSUReq struct {
	PDUSessionID                           int64                                   `json:"pdu_session_id"`
	PDUSessionNASPDU                       *NASPDU                                 `json:"pdu_session_nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI                                  `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer *PDUSessionResourceSetupRequestTransfer `json:"pdu_session_resource_setup_request_transfer,omitempty"`

	Error string `json:"error,omitempty"`
}

// PDU Session Resource Setup Request asks the NG-RAN node to set up user-plane
// resources per session (TS 38.413 §9.2.1.1).
func buildPDUSessionResourceSetupRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceSetupRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Setup Request: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.NASPDU != nil {
		ies = append(ies, ie(ngap.IDNASPDU, ngap.CriticalityReject, libNASPDU(*m.NASPDU)))
	}

	if m.PDUSessionResourceSetup != nil {
		out := make([]PDUSessionResourceSetupSUReq, 0, len(m.PDUSessionResourceSetup))

		for _, item := range m.PDUSessionResourceSetup {
			entry := PDUSessionResourceSetupSUReq{
				PDUSessionID: int64(item.PDUSessionID),
				SNSSAI:       buildSNSSAIValue(item.SNSSAI),
			}

			transfer, err := libPDUSessionResourceSetupRequestTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupRequestTransfer = transfer
			}

			if item.NASPDU != nil {
				entry.PDUSessionNASPDU = ngap.Ptr(libNASPDU(*item.NASPDU))
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceSetupListSUReq, ngap.CriticalityReject, out))
	}

	if m.UEAggregateMaximumBitRate != nil {
		ies = append(ies, ie(ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityIgnore, UEAggregateMaximumBitRate{
			Downlink: int64(m.UEAggregateMaximumBitRate.DL),
			Uplink:   int64(m.UEAggregateMaximumBitRate.UL),
			Unit:     "bps",
		}))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
