// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// AttachExpectReject sends an Attach Request and expects an ATTACH REJECT without
// authentication (e.g. an unknown IMSI, TS 24.301 §5.5.1.2.5), returning the EMM
// cause it carries.
func (e *ENB) AttachExpectReject(ue *UE, timeout time.Duration) (uint8, error) {
	enbUEID := e.AllocateENBUEIDFor(ue)

	attachReq, err := ue.buildAttachRequest()
	if err != nil {
		return 0, err
	}

	if err := e.SendInitialUEMessage(enbUEID, attachReq); err != nil {
		return 0, err
	}

	downlink, _, err := e.WaitForDownlinkNAS(enbUEID, timeout)
	if err != nil {
		return 0, fmt.Errorf("await Attach Reject: %w", err)
	}

	reject, err := expectDownlink[*eps.AttachReject](downlink)
	if err != nil {
		return 0, fmt.Errorf("await Attach Reject: %w", err)
	}

	return uint8(reject.Cause), nil
}

// AttachExpectAuthReject answers the Authentication Request expecting an
// AUTHENTICATION REJECT, the MME's response when RES does not match (TS 24.301
// §5.4.2.5). ue must hold credentials that do not match the provisioned subscriber.
func (e *ENB) AttachExpectAuthReject(ue *UE, timeout time.Duration) error {
	enbUEID := e.AllocateENBUEIDFor(ue)

	attachReq, err := ue.buildAttachRequest()
	if err != nil {
		return err
	}

	if err := e.SendInitialUEMessage(enbUEID, attachReq); err != nil {
		return err
	}

	authNAS, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, timeout)
	if err != nil {
		return fmt.Errorf("await Authentication Request: %w", err)
	}

	if _, err := expectDownlink[*eps.AuthenticationRequest](authNAS); err != nil {
		return fmt.Errorf("await Authentication Request: %w", err)
	}

	authResp, err := ue.handleAuthenticationRequest(authNAS)
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, authResp); err != nil {
		return err
	}

	downlink, _, err := e.WaitForDownlinkNAS(enbUEID, timeout)
	if err != nil {
		return fmt.Errorf("await Authentication Reject: %w", err)
	}

	return validateAuthenticationReject(downlink)
}

// validateAuthenticationReject checks that the PDU is a plain (unprotected)
// AUTHENTICATION REJECT, which carries no IEs beyond its header (TS 24.301 §8.2.5).
func validateAuthenticationReject(pdu []byte) error {
	_, err := expectDownlink[*eps.AuthenticationReject](pdu)

	return err
}

func (e *ENB) AttachExpectESMReject(ue *UE, timeout time.Duration) (*eps.AttachReject, *eps.PDNConnectivityReject, error) {
	enbUEID, _, err := e.attachUntilSecured(ue, timeout)
	if err != nil {
		return nil, nil, err
	}

	frame, err := e.WaitForAnyMessage(enbUEID, Initiating, []s1ap.ProcedureCode{s1ap.ProcDownlinkNASTransport, s1ap.ProcInitialContextSetup}, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("await Attach Reject: %w", err)
	}

	if frame.ProcedureCode != s1ap.ProcDownlinkNASTransport {
		return nil, nil, fmt.Errorf("attach accepted, want Attach Reject")
	}

	dl, err := s1ap.ParseDownlinkNASTransport(frame.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Downlink NAS Transport: %w", err)
	}

	plain, err := ue.unprotectDownlink([]byte(dl.NASPDU))
	if err != nil {
		return nil, nil, fmt.Errorf("unprotect Attach Reject: %w", err)
	}

	reject, err := expectDownlink[*eps.AttachReject](plain)
	if err != nil {
		return nil, nil, fmt.Errorf("await Attach Reject: %w", err)
	}

	esm, err := eps.ParsePDNConnectivityReject(reject.ESMMessageContainer)
	if err != nil {
		return nil, nil, fmt.Errorf("attach reject with EMM cause %d carries no PDN Connectivity Reject: %w", reject.Cause, err)
	}

	if err := e.completeContextRelease(enbUEID, timeout); err != nil {
		return nil, nil, err
	}

	return reject, esm, nil
}
