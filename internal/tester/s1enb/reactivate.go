// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// ReactivateBearer waits for the MME's DEACTIVATE EPS BEARER CONTEXT REQUEST,
// acknowledges it with a DEACTIVATE EPS BEARER CONTEXT ACCEPT, and completes the
// S1 context release the MME triggers — the EPS reaction to a data-network
// reconfiguration (TS 24.301 §6.4.4.2). It returns the parsed request so the
// caller can assert ESM cause #39 "reactivation requested". Proactive downlink
// NAS the MME interleaves (e.g. EMM INFORMATION) is skipped, as a real UE would.
// Deactivating the last default bearer leaves the UE EMM-DEREGISTERED, so it
// re-attaches afterward to pick up the new configuration.
func (e *ENB) ReactivateBearer(ue *UE, enbUEID int64, timeout time.Duration) (*eps.DeactivateEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timed out awaiting Deactivate EPS Bearer Context Request")
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(remaining)
		if err != nil {
			return nil, err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return nil, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		mt, err := eps.PeekESMMessageType(plain)
		if err != nil || mt != eps.MsgDeactivateEPSBearerContextRequest {
			continue
		}

		req, err := eps.ParseDeactivateEPSBearerContextRequest(plain)
		if err != nil {
			return nil, fmt.Errorf("parse Deactivate EPS Bearer Context Request: %w", err)
		}

		accept, err := ue.buildDeactivateEPSBearerContextAccept(req.EPSBearerIdentity, req.ProcedureTransactionIdentity)
		if err != nil {
			return nil, err
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
			return nil, fmt.Errorf("send Deactivate EPS Bearer Context Accept: %w", err)
		}

		if err := e.completeContextRelease(time.Until(deadline)); err != nil {
			return nil, err
		}

		return req, nil
	}
}

// ModifyBearer awaits the MME's MODIFY EPS BEARER CONTEXT REQUEST and
// acknowledges it with a MODIFY EPS BEARER CONTEXT ACCEPT — the EPS reaction to
// an in-place data-network change (a DNS update) that does not re-establish the
// bearer (TS 24.301 §6.4.2). It returns the parsed request so the caller can
// assert the new DNS in the Protocol Configuration Options. Proactive downlink
// NAS (e.g. EMM INFORMATION) is skipped, as a real UE would.
func (e *ENB) ModifyBearer(ue *UE, enbUEID int64, timeout time.Duration) (*eps.ModifyEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("timed out awaiting Modify EPS Bearer Context Request")
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(remaining)
		if err != nil {
			return nil, err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return nil, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		mt, err := eps.PeekESMMessageType(plain)
		if err != nil || mt != eps.MsgModifyEPSBearerContextRequest {
			continue
		}

		req, err := eps.ParseModifyEPSBearerContextRequest(plain)
		if err != nil {
			return nil, fmt.Errorf("parse Modify EPS Bearer Context Request: %w", err)
		}

		accept, err := ue.buildModifyEPSBearerContextAccept(req.EPSBearerIdentity, req.ProcedureTransactionIdentity)
		if err != nil {
			return nil, err
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
			return nil, fmt.Errorf("send Modify EPS Bearer Context Accept: %w", err)
		}

		return req, nil
	}
}

// buildDeactivateEPSBearerContextAccept builds the protected DEACTIVATE EPS
// BEARER CONTEXT ACCEPT acknowledging a network-initiated deactivation
// (TS 24.301 §8.3.9).
func (ue *UE) buildDeactivateEPSBearerContextAccept(ebi, pti uint8) ([]byte, error) {
	plain, err := (&eps.DeactivateEPSBearerContextAccept{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
	}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Deactivate EPS Bearer Context Accept: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Deactivate EPS Bearer Context Accept: %w", err)
	}

	ue.ulCount++

	return out, nil
}

// buildModifyEPSBearerContextAccept builds the protected MODIFY EPS BEARER
// CONTEXT ACCEPT acknowledging a network-initiated modification (TS 24.301
// §8.3.19).
func (ue *UE) buildModifyEPSBearerContextAccept(ebi, pti uint8) ([]byte, error) {
	plain, err := (&eps.ModifyEPSBearerContextAccept{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
	}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Modify EPS Bearer Context Accept: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Modify EPS Bearer Context Accept: %w", err)
	}

	ue.ulCount++

	return out, nil
}
