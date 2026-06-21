// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// ModifyBearerViaERABModify waits for an S1AP E-RAB Modify Request (the QCI/ARP
// modification path, TS 36.413 §8.2.2), replies with an E-RAB Modify Response,
// relays the piggybacked MODIFY EPS BEARER CONTEXT REQUEST to the UE with a Modify
// Accept, and returns both the E-RAB Modify Request (for the E-RAB-level QoS) and
// the parsed NAS message (for the EPS QoS / APN-AMBR IEs).
func (e *ENB) ModifyBearerViaERABModify(ue *UE, enbUEID int64, timeout time.Duration) (*s1ap.ERABModifyRequest, *eps.ModifyEPSBearerContextRequest, error) {
	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcERABModify, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("await E-RAB Modify Request: %w", err)
	}

	req, err := s1ap.ParseERABModifyRequest(frame.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("parse E-RAB Modify Request: %w", err)
	}

	if len(req.ERABToBeModified) == 0 {
		return nil, nil, fmt.Errorf("E-RAB Modify Request without an E-RAB")
	}

	if err := e.sendERABModifyResponse(req, enbUEID); err != nil {
		return nil, nil, err
	}

	plain, err := ue.unprotectDownlink([]byte(req.ERABToBeModified[0].NASPDU))
	if err != nil {
		return nil, nil, fmt.Errorf("unprotect piggybacked NAS: %w", err)
	}

	nasReq, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Modify EPS Bearer Context Request: %w", err)
	}

	accept, err := ue.buildModifyEPSBearerContextAccept(nasReq.EPSBearerIdentity, nasReq.ProcedureTransactionIdentity)
	if err != nil {
		return nil, nil, err
	}

	if err := e.SendUplinkNASTransport(int64(req.MMEUES1APID), enbUEID, accept); err != nil {
		return nil, nil, fmt.Errorf("send Modify EPS Bearer Context Accept: %w", err)
	}

	return req, nasReq, nil
}

func (e *ENB) sendERABModifyResponse(req *s1ap.ERABModifyRequest, enbUEID int64) error {
	resp := &s1ap.ERABModifyResponse{
		MMEUES1APID: req.MMEUES1APID,
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		ERABModify:  []s1ap.ERABModifyItemBearerModRes{{ERABID: req.ERABToBeModified[0].ERABID}},
	}

	b, err := resp.Marshal()
	if err != nil {
		return fmt.Errorf("build E-RAB Modify Response: %w", err)
	}

	return e.SendMessage(b, true)
}
