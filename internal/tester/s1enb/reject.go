// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

// AttachExpectReject sends an Attach Request and expects an ATTACH REJECT without
// authentication (e.g. an unknown IMSI, TS 24.301 §5.5.1.2.5), returning the EMM
// cause it carries.
func (e *ENB) AttachExpectReject(ue *UE, timeout time.Duration) (uint8, error) {
	enbUEID := e.AllocateENBUEID()

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

// AttachExpectESMReject drives a full authenticated attach and expects the
// network to refuse its ESM procedure. It returns the EMM cause of the ATTACH
// REJECT and the ESM cause of the PDN CONNECTIVITY REJECT it carries; the pair
// is #19 "ESM failure" plus the ESM cause (TS 24.301 §5.5.1.2.5).
//
// Unlike AttachExpectReject, which covers a refusal before security is
// activated, this reject arrives integrity protected and ciphered
// (TS 24.301 §4.4.4.2).
func (e *ENB) AttachExpectESMReject(ue *UE, timeout time.Duration) (eps.EMMCause, eps.ESMCause, error) {
	session, err := e.authenticateAttach(ue, timeout)
	if err != nil {
		return 0, 0, err
	}

	wire, _, err := e.WaitForDownlinkNAS(session.enbUEID, timeout)
	if err != nil {
		return 0, 0, fmt.Errorf("await Attach Reject: %w", err)
	}

	plain, err := ue.unprotectDownlink(wire)
	if err != nil {
		return 0, 0, fmt.Errorf("unprotect Attach Reject: %w", err)
	}

	reject, err := expectDownlink[*eps.AttachReject](plain)
	if err != nil {
		return 0, 0, fmt.Errorf("await Attach Reject: %w", err)
	}

	if len(reject.ESMMessageContainer) == 0 {
		return reject.Cause, 0, fmt.Errorf("Attach Reject carries no ESM message container")
	}

	esmReject, err := eps.ParsePDNConnectivityReject(reject.ESMMessageContainer)
	if err != nil {
		return reject.Cause, 0, fmt.Errorf("parse PDN Connectivity Reject: %w", err)
	}

	return reject.Cause, esmReject.Cause, nil
}

// AttachExpectAuthReject answers the Authentication Request expecting an
// AUTHENTICATION REJECT, the MME's response when RES does not match (TS 24.301
// §5.4.2.5). ue must hold credentials that do not match the provisioned subscriber.
func (e *ENB) AttachExpectAuthReject(ue *UE, timeout time.Duration) error {
	enbUEID := e.AllocateENBUEID()

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
