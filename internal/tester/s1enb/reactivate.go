// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// ReactivateBearer handles a network-initiated bearer deactivation, the EPS
// reaction to a data-network reconfiguration (TS 24.301 §6.4.4.2). It returns the
// parsed request so the caller can assert ESM cause #39 "reactivation requested".
// Proactive downlink NAS the MME interleaves (e.g. EMM INFORMATION) is skipped, as
// a real UE would.
func (e *ENB) ReactivateBearer(ue *UE, enbUEID int64, timeout time.Duration) (*eps.DeactivateEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timed out awaiting Deactivate EPS Bearer Context Request")
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return nil, err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return nil, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		msg, err := parseDownlink(plain)
		if err != nil {
			return nil, err
		}

		req, ok := msg.(*eps.DeactivateEPSBearerContextRequest)
		if !ok {
			continue
		}

		accept, err := ue.buildDeactivateEPSBearerContextAccept(uint8(req.EPSBearerIdentity), uint8(req.PTI))
		if err != nil {
			return nil, err
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
			return nil, fmt.Errorf("send Deactivate EPS Bearer Context Accept: %w", err)
		}

		if err := e.completeContextRelease(enbUEID, time.Until(deadline)); err != nil {
			return nil, err
		}

		return req, nil
	}
}

// ModifyBearer handles a network-initiated bearer modification, the EPS reaction
// to an in-place data-network change (a DNS update) that does not re-establish the
// bearer (TS 24.301 §6.4.2). It returns the parsed request so the caller can assert
// the new DNS in the Protocol Configuration Options. Proactive downlink NAS (e.g.
// EMM INFORMATION) is skipped, as a real UE would.
func (e *ENB) ModifyBearer(ue *UE, enbUEID int64, timeout time.Duration) (*eps.ModifyEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timed out awaiting Modify EPS Bearer Context Request")
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return nil, err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return nil, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		msg, err := parseDownlink(plain)
		if err != nil {
			return nil, err
		}

		req, ok := msg.(*eps.ModifyEPSBearerContextRequest)
		if !ok {
			continue
		}

		accept, err := ue.buildModifyEPSBearerContextAccept(uint8(req.EPSBearerIdentity), uint8(req.PTI))
		if err != nil {
			return nil, err
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
			return nil, fmt.Errorf("send Modify EPS Bearer Context Accept: %w", err)
		}

		return req, nil
	}
}

func (ue *UE) buildDeactivateEPSBearerContextAccept(ebi, pti uint8) ([]byte, error) {
	plain, err := (&eps.DeactivateEPSBearerContextAccept{
		EPSBearerIdentity: eps.EPSBearerIdentity(ebi),
		PTI:               nas.ProcedureTransactionIdentity(pti),
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("build Deactivate EPS Bearer Context Accept: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nas.MakeCount(0, ue.ulCount), nas.DirectionUplink, ue.sc)
	if err != nil {
		return nil, fmt.Errorf("protect Deactivate EPS Bearer Context Accept: %w", err)
	}

	ue.ulCount++

	return out, nil
}

func (ue *UE) buildModifyEPSBearerContextAccept(ebi, pti uint8) ([]byte, error) {
	plain, err := (&eps.ModifyEPSBearerContextAccept{
		EPSBearerIdentity: eps.EPSBearerIdentity(ebi),
		PTI:               nas.ProcedureTransactionIdentity(pti),
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("build Modify EPS Bearer Context Accept: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nas.MakeCount(0, ue.ulCount), nas.DirectionUplink, ue.sc)
	if err != nil {
		return nil, fmt.Errorf("protect Modify EPS Bearer Context Accept: %w", err)
	}

	ue.ulCount++

	return out, nil
}
