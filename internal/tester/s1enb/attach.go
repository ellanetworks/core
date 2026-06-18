// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// AttachResult reports the identifiers and GUTI established by a completed attach.
type AttachResult struct {
	MMEUES1APID int64
	ENBUES1APID int64
	ERABID      s1ap.ERABID // the default bearer's E-RAB ID
	GUTI        *eps.EPSMobileIdentity

	// IdentityRequested is set when the MME answered the Attach Request with an
	// Identity Request (an unresolvable GUTI, TS 24.301 §5.4.4).
	IdentityRequested bool

	// PDNType is the PDN type negotiated for the default bearer in the Attach
	// Accept (eps.PDNTypeIPv4 / IPv6 / IPv4v6).
	PDNType uint8

	// User-plane endpoints for a GTP-U tunnel.
	UEIPv4     string // UE IPv4 assigned in the Attach Accept
	UEIPv6     string // UE IPv6 link-local derived from the Attach Accept PDN IID
	UpfAddress string // S-GW/UPF S1-U address (uplink target)
	ULTEID     uint32 // S-GW/UPF uplink TEID
	DLTEID     uint32 // eNB downlink TEID reported to the MME
}

// Attach drives a full EPS attach for ue (TS 24.301 §5.5.1.2): Attach Request →
// EPS-AKA authentication → Security Mode Control → Initial Context Setup (with
// the Attach Accept) → Attach Complete. It returns once Attach Complete is sent.
func (e *ENB) Attach(ue *UE, timeout time.Duration) (*AttachResult, error) {
	enbUEID := e.AllocateENBUEID()

	attachReq, err := ue.buildAttachRequest()
	if err != nil {
		return nil, err
	}

	if err := e.SendInitialUEMessage(enbUEID, attachReq); err != nil {
		return nil, err
	}

	// 1. An unresolvable GUTI draws an Identity Request first (TS 24.301 §5.4.4);
	//    answer it with the IMSI, then expect the Authentication Request. The first
	//    NAS is plain, so skip any protected frame left by a prior UE on the same
	//    association (e.g. its post-attach EMM INFORMATION) that this UE cannot
	//    decrypt — this keeps sequential multi-UE attach on one eNB clean.
	downlink, mmeUEID, err := e.waitForPlainDownlink(timeout)
	if err != nil {
		return nil, fmt.Errorf("await Authentication/Identity Request: %w", err)
	}

	identityRequested := false

	if mt, err := eps.PeekMessageType(downlink); err == nil && mt == eps.MsgIdentityRequest {
		identityRequested = true

		idResp, err := ue.buildIdentityResponse()
		if err != nil {
			return nil, err
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, idResp); err != nil {
			return nil, err
		}

		if downlink, _, err = e.WaitForDownlinkNAS(timeout); err != nil {
			return nil, fmt.Errorf("await Authentication Request: %w", err)
		}
	}

	// 2. Authentication Request (plain downlink) → Authentication Response.
	if mt, err := eps.PeekMessageType(downlink); err != nil || mt != eps.MsgAuthenticationRequest {
		return nil, fmt.Errorf("expected Authentication Request, got message type %#x (err %v)", mt, err)
	}

	authResp, err := ue.handleAuthenticationRequest(downlink)
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, authResp); err != nil {
		return nil, err
	}

	// 3. Security Mode Command (protected downlink) → Security Mode Complete.
	smcWire, _, err := e.WaitForDownlinkNAS(timeout)
	if err != nil {
		return nil, fmt.Errorf("await Security Mode Command: %w", err)
	}

	smcComplete, err := ue.handleSecurityModeCommand(smcWire)
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, smcComplete); err != nil {
		return nil, err
	}

	// 4. Initial Context Setup Request carries the Attach Accept; reply with the
	//    Initial Context Setup Response and process the Attach Accept.
	icsFrame, err := e.WaitForMessage(Initiating, s1ap.ProcInitialContextSetup, timeout)
	if err != nil {
		return nil, fmt.Errorf("await Initial Context Setup Request: %w", err)
	}

	ics, err := s1ap.ParseInitialContextSetupRequest(icsFrame.Value)
	if err != nil {
		return nil, fmt.Errorf("parse Initial Context Setup Request: %w", err)
	}

	if len(ics.ERABToBeSetup) == 0 {
		return nil, fmt.Errorf("initial context setup request without an E-RAB")
	}

	// The E-RAB carries the S-GW/UPF S1-U endpoint (the uplink target); the eNB
	// reports its own downlink endpoint in the response.
	erab := ics.ERABToBeSetup[0]

	upf, err := e.selectUpfAddr(erab.TransportLayerAddress)
	if err != nil {
		return nil, err
	}

	dlTEID := e.allocTEID()

	if err := e.sendInitialContextSetupResponse(ics, enbUEID, dlTEID); err != nil {
		return nil, err
	}

	acceptPlain, err := ue.unprotectDownlink([]byte(ics.ERABToBeSetup[0].NASPDU))
	if err != nil {
		return nil, fmt.Errorf("unprotect Attach Accept: %w", err)
	}

	accept, err := eps.ParseAttachAccept(acceptPlain)
	if err != nil {
		return nil, fmt.Errorf("parse Attach Accept: %w", err)
	}

	// 5. Attach Complete.
	attachComplete, err := ue.buildAttachComplete(accept.ESMMessageContainer)
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, attachComplete); err != nil {
		return nil, err
	}

	logger.GnbLogger.Debug("Attach complete",
		zap.String("imsi", ue.IMSI), zap.Int64("mme-ue-id", mmeUEID), zap.Int64("enb-ue-id", enbUEID))

	res := &AttachResult{
		MMEUES1APID:       mmeUEID,
		ENBUES1APID:       enbUEID,
		ERABID:            erab.ERABID,
		GUTI:              accept.GUTI,
		IdentityRequested: identityRequested,
		UpfAddress:        upf.Unmap().String(),
		ULTEID:            uint32(erab.GTPTEID),
		DLTEID:            dlTEID,
	}

	if act, err := eps.ParseActivateDefaultEPSBearerContextRequest(accept.ESMMessageContainer); err == nil {
		if pdn, err := eps.ParsePDNAddress(act.PDNAddress); err == nil {
			res.PDNType = pdn.PDNType

			if pdn.IPv4 != ([4]byte{}) {
				res.UEIPv4 = netip.AddrFrom4(pdn.IPv4).String()
			}

			if pdn.IPv6IID != ([8]byte{}) {
				// The PDN address carries only the interface identifier; the UE
				// forms a link-local from it and obtains the global prefix via
				// the UPF's Router Advertisement (SLAAC).
				a := [16]byte{0: 0xfe, 1: 0x80}
				copy(a[8:], pdn.IPv6IID[:])
				res.UEIPv6 = netip.AddrFrom16(a).String()
			}
		}
	}

	return res, nil
}

// waitForPlainDownlink returns the first plain (unprotected) downlink NAS,
// skipping protected frames left from a prior UE on the same association (e.g.
// the proactive EMM INFORMATION), which a fresh UE cannot decrypt. The first NAS
// of an attach (Identity or Authentication Request) is always plain.
func (e *ENB) waitForPlainDownlink(timeout time.Duration) (nas []byte, mmeUEID int64, err error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, 0, fmt.Errorf("s1enb: timeout waiting for a plain downlink NAS")
		}

		nas, mmeUEID, err = e.WaitForDownlinkNAS(remaining)
		if err != nil {
			return nil, 0, err
		}

		if len(nas) > 0 && nas[0]>>4 == uint8(eps.SHTPlain) {
			return nas, mmeUEID, nil
		}
	}
}

// sendInitialContextSetupResponse acknowledges the bearer setup, reporting the
// eNB's S1-U downlink endpoint (the F-TEID the MME hands to the UPF).
func (e *ENB) sendInitialContextSetupResponse(ics *s1ap.InitialContextSetupRequest, enbUEID int64, dlTEID uint32) error {
	addr := e.n3Addr.To4()
	if addr == nil {
		addr = e.n3Addr.To16()
	}

	resp := &s1ap.InitialContextSetupResponse{
		MMEUES1APID: ics.MMEUES1APID,
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		ERABSetup: []s1ap.ERABSetupItemCtxtSURes{{
			ERABID:                ics.ERABToBeSetup[0].ERABID,
			TransportLayerAddress: s1ap.TransportLayerAddress(addr),
			GTPTEID:               s1ap.GTPTEID(dlTEID),
		}},
	}

	b, err := resp.Marshal()
	if err != nil {
		return fmt.Errorf("build Initial Context Setup Response: %w", err)
	}

	return e.SendMessage(b, true)
}

// selectUpfAddr resolves the UPF S1-U endpoint from the E-RAB Transport Layer
// Address (TS 36.413): 4 octets IPv4, 16 IPv6, or 20 dual-stack (IPv4||IPv6).
// For a dual-stack endpoint it picks the family of the eNB's own N3 socket, since
// the GTP-U transport between eNB and UPF must share an address family.
func (e *ENB) selectUpfAddr(tla s1ap.TransportLayerAddress) (netip.Addr, error) {
	b := []byte(tla)
	enbIsV6 := e.n3Addr.To4() == nil

	var v4, v6 netip.Addr

	switch len(b) {
	case 4:
		v4, _ = netip.AddrFromSlice(b)
	case 16:
		v6, _ = netip.AddrFromSlice(b)
	case 20:
		v4, _ = netip.AddrFromSlice(b[0:4])
		v6, _ = netip.AddrFromSlice(b[4:20])
	default:
		return netip.Addr{}, fmt.Errorf("s1enb: unexpected S1-U transport layer address length %d", len(b))
	}

	switch {
	case enbIsV6 && v6.IsValid():
		return v6.Unmap(), nil
	case !enbIsV6 && v4.IsValid():
		return v4.Unmap(), nil
	case v4.IsValid():
		return v4.Unmap(), nil
	case v6.IsValid():
		return v6.Unmap(), nil
	default:
		return netip.Addr{}, fmt.Errorf("s1enb: no usable S1-U transport address")
	}
}

func (e *ENB) allocTEID() uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()

	t := e.nextTEID
	e.nextTEID++

	return t
}
