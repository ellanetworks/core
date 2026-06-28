// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// PDNResult reports the bearer established for an additional PDN connection,
// including the user-plane endpoints for a GTP-U tunnel.
type PDNResult struct {
	ERABID              s1ap.ERABID
	PDNType             uint8
	QCI                 byte   // default bearer QCI for this PDN (== policy 5QI)
	ARP                 byte   // E-RAB ARP priority level (1-15, TS 36.413 §9.2.1.60)
	APN                 string // Access Point Name of this PDN connection
	SessAmbrDownlinkBps uint64 // per-APN Session-AMBR from the APN-AMBR IE (bits/s)
	SessAmbrUplinkBps   uint64
	UEIPv4              string
	UEIPv6              string
	UpfAddress          string
	ULTEID              uint32
	DLTEID              uint32
}

// OpenPDN opens an additional PDN connection to apn for an attached UE (TS 24.301
// §6.5.1), returning the new bearer's user-plane endpoints.
func (e *ENB) OpenPDN(ue *UE, mmeUEID, enbUEID int64, apn string, pdnType uint8, timeout time.Duration) (*PDNResult, error) {
	apnIE, err := eps.MarshalAPN(apn)
	if err != nil {
		return nil, fmt.Errorf("encode APN: %w", err)
	}

	connReq, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: ue.nextPTI(),
		RequestType:                  1, // initial request
		PDNType:                      pdnType,
		AccessPointName:              apnIE,
	}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build PDN Connectivity Request: %w", err)
	}

	protected, err := ue.protectUplink(connReq)
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, protected); err != nil {
		return nil, fmt.Errorf("send PDN Connectivity Request: %w", err)
	}

	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcERABSetup, timeout)
	if err != nil {
		return nil, fmt.Errorf("await E-RAB Setup Request: %w", err)
	}

	req, err := s1ap.ParseERABSetupRequest(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("parse E-RAB Setup Request: %w", err)
	}

	if len(req.ERABToBeSetup) == 0 {
		return nil, fmt.Errorf("E-RAB Setup Request without an E-RAB")
	}

	erab := req.ERABToBeSetup[0]

	upf, err := e.selectUpfAddr(erab.TransportLayerAddress)
	if err != nil {
		return nil, err
	}

	dlTEID := e.allocTEID()

	if err := e.sendERABSetupResponse(req, enbUEID, dlTEID); err != nil {
		return nil, err
	}

	actPlain, err := ue.unprotectDownlink([]byte(erab.NASPDU))
	if err != nil {
		return nil, fmt.Errorf("unprotect Activate Default EPS Bearer Context Request: %w", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(actPlain)
	if err != nil {
		return nil, fmt.Errorf("parse Activate Default EPS Bearer Context Request: %w", err)
	}

	accept, err := ue.protectUplink([]byte{0x02, act.ProcedureTransactionIdentity, uint8(eps.MsgActivateDefaultEPSBearerContextAccept)})
	if err != nil {
		return nil, err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
		return nil, fmt.Errorf("send Activate Default EPS Bearer Context Accept: %w", err)
	}

	res := &PDNResult{
		ERABID:     erab.ERABID,
		ARP:        erab.QoS.ARP.PriorityLevel,
		UpfAddress: upf.Unmap().String(),
		ULTEID:     uint32(erab.GTPTEID),
		DLTEID:     dlTEID,
	}

	if len(act.EPSQoS) >= 1 {
		res.QCI = act.EPSQoS[0]
	}

	if apn, err := eps.ParseAPN(act.AccessPointName); err == nil {
		res.APN = apn
	}

	if len(act.APNAMBR) > 0 {
		if ambr, err := eps.ParseAPNAMBR(act.APNAMBR); err == nil {
			res.SessAmbrDownlinkBps, res.SessAmbrUplinkBps = ambr.BitsPerSecond()
		}
	}

	if pdn, err := eps.ParsePDNAddress(act.PDNAddress); err == nil {
		res.PDNType = pdn.PDNType

		if pdn.IPv4 != ([4]byte{}) {
			res.UEIPv4 = netip.AddrFrom4(pdn.IPv4).String()
		}

		if pdn.IPv6IID != ([8]byte{}) {
			a := [16]byte{0: 0xfe, 1: 0x80}
			copy(a[8:], pdn.IPv6IID[:])
			res.UEIPv6 = netip.AddrFrom16(a).String()
		}
	}

	return res, nil
}

// DisconnectPDN releases an additional PDN connection (TS 24.301 §6.5.2; TS
// 23.401 §5.10.3).
func (e *ENB) DisconnectPDN(ue *UE, mmeUEID, enbUEID int64, linkedEBI uint8, timeout time.Duration) error {
	disc, err := (&eps.PDNDisconnectRequest{
		ProcedureTransactionIdentity: ue.nextPTI(),
		LinkedEPSBearerIdentity:      linkedEBI,
	}).Marshal()
	if err != nil {
		return fmt.Errorf("build PDN Disconnect Request: %w", err)
	}

	protected, err := ue.protectUplink(disc)
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, protected); err != nil {
		return fmt.Errorf("send PDN Disconnect Request: %w", err)
	}

	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcERABRelease, timeout)
	if err != nil {
		return fmt.Errorf("await E-RAB Release Command: %w", err)
	}

	cmd, err := s1ap.ParseERABReleaseCommand(frame.Value)
	if err != nil {
		return fmt.Errorf("parse E-RAB Release Command: %w", err)
	}

	if len(cmd.ERABToBeReleased) == 0 {
		return fmt.Errorf("E-RAB Release Command without an E-RAB")
	}

	if err := e.sendERABReleaseResponse(cmd, enbUEID); err != nil {
		return err
	}

	plain, err := ue.unprotectDownlink([]byte(cmd.NASPDU))
	if err != nil {
		return fmt.Errorf("unprotect Deactivate EPS Bearer Context Request: %w", err)
	}

	reqMsg, err := eps.ParseDeactivateEPSBearerContextRequest(plain)
	if err != nil {
		return fmt.Errorf("parse Deactivate EPS Bearer Context Request: %w", err)
	}

	accept, err := ue.buildDeactivateEPSBearerContextAccept(reqMsg.EPSBearerIdentity, reqMsg.ProcedureTransactionIdentity)
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, accept); err != nil {
		return fmt.Errorf("send Deactivate EPS Bearer Context Accept: %w", err)
	}

	return nil
}

// sendERABReleaseResponse acknowledges an E-RAB RELEASE COMMAND, confirming the
// listed E-RABs are released (TS 36.413 §8.2.3).
func (e *ENB) sendERABReleaseResponse(cmd *s1ap.ERABReleaseCommand, enbUEID int64) error {
	released := make([]s1ap.ERABReleaseItemBearerRelComp, len(cmd.ERABToBeReleased))
	for i, erab := range cmd.ERABToBeReleased {
		released[i] = s1ap.ERABReleaseItemBearerRelComp{ERABID: erab.ERABID}
	}

	resp := &s1ap.ERABReleaseResponse{
		MMEUES1APID:  cmd.MMEUES1APID,
		ENBUES1APID:  s1ap.ENBUES1APID(enbUEID),
		ERABReleased: released,
	}

	b, err := resp.Marshal()
	if err != nil {
		return fmt.Errorf("build E-RAB Release Response: %w", err)
	}

	return e.SendMessage(b, true)
}

// sendERABSetupResponse acknowledges an E-RAB SETUP REQUEST, reporting the eNB's
// S1-U downlink endpoint for the new E-RAB (TS 36.413 §8.2.1).
func (e *ENB) sendERABSetupResponse(req *s1ap.ERABSetupRequest, enbUEID int64, dlTEID uint32) error {
	addr := e.n3Addr.To4()
	if addr == nil {
		addr = e.n3Addr.To16()
	}

	resp := &s1ap.ERABSetupResponse{
		MMEUES1APID: req.MMEUES1APID,
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		ERABSetup: []s1ap.ERABSetupItemBearerSURes{{
			ERABID:                req.ERABToBeSetup[0].ERABID,
			TransportLayerAddress: s1ap.TransportLayerAddress(addr),
			GTPTEID:               s1ap.GTPTEID(dlTEID),
		}},
	}

	b, err := resp.Marshal()
	if err != nil {
		return fmt.Errorf("build E-RAB Setup Response: %w", err)
	}

	return e.SendMessage(b, true)
}
