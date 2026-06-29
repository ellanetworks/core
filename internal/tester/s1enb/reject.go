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

	reject, err := eps.ParseAttachReject(downlink)
	if err != nil {
		mt, _ := eps.PeekMessageType(downlink)
		return 0, fmt.Errorf("expected Attach Reject, got message type %#x: %w", mt, err)
	}

	return reject.Cause, nil
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

	if mt, err := eps.PeekMessageType(authNAS); err != nil || mt != eps.MsgAuthenticationRequest {
		return fmt.Errorf("expected Authentication Request, got message type %#x (err %v)", mt, err)
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

// validateAuthenticationReject checks the AUTHENTICATION REJECT header: a plain
// (unprotected) EMM message of type Authentication Reject (TS 24.301 §8.2.5),
// which carries no IEs beyond the two-octet header.
func validateAuthenticationReject(nas []byte) error {
	if len(nas) < 2 {
		return fmt.Errorf("authentication reject too short: %d bytes", len(nas))
	}

	if pd := nas[0] & 0x0F; pd != eps.PDEMM {
		return fmt.Errorf("authentication reject protocol discriminator = %#x, want EMM %#x", pd, eps.PDEMM)
	}

	if sht := eps.SecurityHeaderType(nas[0] >> 4); sht != eps.SHTPlain {
		return fmt.Errorf("authentication reject security header type = %d, want plain", sht)
	}

	if mt := eps.MessageType(nas[1]); mt != eps.MsgAuthenticationReject {
		return fmt.Errorf("expected Authentication Reject, got message type %#x", mt)
	}

	return nil
}
