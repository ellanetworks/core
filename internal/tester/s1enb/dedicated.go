// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"slices"
	"time"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

type DedicatedBearer struct {
	ERABID    s1ap.ERABID
	LinkedEBI uint8
	QoS       s1ap.ERABLevelQoSParameters
	EPSQoS    eps.EPSQoS
	TFT       eps.TrafficFlowTemplate
	ULTEID    uint32
	DLTEID    uint32
}

func (e *ENB) awaitDedicatedActivation(ue *UE, enbUEID int64, timeout time.Duration) (*s1ap.ERABSetupRequest, *eps.ActivateDedicatedEPSBearerContextRequest, error) {
	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcERABSetup, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("await E-RAB Setup Request: %w", err)
	}

	req, err := s1ap.ParseERABSetupRequest(frame.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("parse E-RAB Setup Request: %w", err)
	}

	if len(req.ERABToBeSetup) != 1 {
		return nil, nil, fmt.Errorf("E-RAB Setup Request with %d E-RABs, want 1", len(req.ERABToBeSetup))
	}

	plain, err := ue.unprotectDownlink([]byte(req.ERABToBeSetup[0].NASPDU))
	if err != nil {
		return nil, nil, fmt.Errorf("unprotect Activate Dedicated EPS Bearer Context Request: %w", err)
	}

	act, err := eps.ParseActivateDedicatedEPSBearerContextRequest(plain)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Activate Dedicated EPS Bearer Context Request: %w", err)
	}

	return req, act, nil
}

func (e *ENB) AcceptDedicatedBearer(ue *UE, enbUEID int64, timeout time.Duration) (*DedicatedBearer, error) {
	req, act, err := e.awaitDedicatedActivation(ue, enbUEID, timeout)
	if err != nil {
		return nil, err
	}

	erab := req.ERABToBeSetup[0]
	dlTEID := e.allocTEID()

	if err := e.sendERABSetupResponse(req, enbUEID, dlTEID); err != nil {
		return nil, err
	}

	plain, err := (&eps.ActivateDedicatedEPSBearerContextAccept{EPSBearerIdentity: act.EPSBearerIdentity, PTI: act.PTI}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("build Activate Dedicated EPS Bearer Context Accept: %w", err)
	}

	if err := e.sendESM(ue, int64(req.MMEUES1APID), enbUEID, plain); err != nil {
		return nil, fmt.Errorf("send Activate Dedicated EPS Bearer Context Accept: %w", err)
	}

	return &DedicatedBearer{
		ERABID:    erab.ERABID,
		LinkedEBI: uint8(act.LinkedEPSBearerIdentity),
		QoS:       erab.QoS,
		EPSQoS:    act.EPSQoS,
		TFT:       act.TFT,
		ULTEID:    uint32(erab.GTPTEID),
		DLTEID:    dlTEID,
	}, nil
}

func (e *ENB) RejectDedicatedBearer(ue *UE, enbUEID int64, timeout time.Duration) (s1ap.ERABID, error) {
	req, act, err := e.awaitDedicatedActivation(ue, enbUEID, timeout)
	if err != nil {
		return 0, err
	}

	if err := e.sendERABSetupResponse(req, enbUEID, e.allocTEID()); err != nil {
		return 0, err
	}

	plain, err := (&eps.ActivateDedicatedEPSBearerContextReject{
		EPSBearerIdentity: act.EPSBearerIdentity,
		PTI:               act.PTI,
		Cause:             eps.ESMCauseInsufficientResources,
	}).MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("build Activate Dedicated EPS Bearer Context Reject: %w", err)
	}

	if err := e.sendESM(ue, int64(req.MMEUES1APID), enbUEID, plain); err != nil {
		return 0, fmt.Errorf("send Activate Dedicated EPS Bearer Context Reject: %w", err)
	}

	return req.ERABToBeSetup[0].ERABID, nil
}

func (e *ENB) AwaitDedicatedDeactivation(ue *UE, enbUEID int64, timeout time.Duration) (s1ap.ERABID, *eps.DeactivateEPSBearerContextRequest, error) {
	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcERABRelease, timeout)
	if err != nil {
		return 0, nil, fmt.Errorf("await E-RAB Release Command: %w", err)
	}

	return e.answerDedicatedRelease(ue, enbUEID, frame)
}

func (e *ENB) AwaitUnmodifiedDedicatedDeactivation(ue *UE, enbUEID int64, timeout time.Duration) (s1ap.ERABID, *eps.DeactivateEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		frame, err := e.WaitForAnyMessage(enbUEID, Initiating, []s1ap.ProcedureCode{s1ap.ProcERABModify, s1ap.ProcDownlinkNASTransport, s1ap.ProcERABRelease}, time.Until(deadline))
		if err != nil {
			return 0, nil, fmt.Errorf("await E-RAB Release Command: %w", err)
		}

		switch frame.ProcedureCode {
		case s1ap.ProcERABRelease:
			return e.answerDedicatedRelease(ue, enbUEID, frame)
		case s1ap.ProcERABModify:
			return 0, nil, fmt.Errorf("the dedicated bearer got an E-RAB Modify Request before its release, want no change")
		}

		dl, err := s1ap.ParseDownlinkNASTransport(frame.Value)
		if err != nil {
			return 0, nil, fmt.Errorf("parse Downlink NAS Transport: %w", err)
		}

		plain, err := ue.unprotectDownlink([]byte(dl.NASPDU))
		if err != nil {
			return 0, nil, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		if _, err := eps.ParseModifyEPSBearerContextRequest(plain); err == nil {
			return 0, nil, fmt.Errorf("the dedicated bearer got a Modify EPS Bearer Context Request before its release, want no change")
		}
	}
}

func (e *ENB) AcceptDedicatedChangesUntilRelease(ue *UE, enbUEID int64, b *DedicatedBearer, timeout time.Duration) (s1ap.ERABID, error) {
	deadline := time.Now().Add(timeout)

	for {
		frame, err := e.WaitForAnyMessage(enbUEID, Initiating, []s1ap.ProcedureCode{s1ap.ProcERABModify, s1ap.ProcDownlinkNASTransport, s1ap.ProcERABRelease}, time.Until(deadline))
		if err != nil {
			return 0, fmt.Errorf("await a change of E-RAB %d: %w", b.ERABID, err)
		}

		if frame.ProcedureCode == s1ap.ProcERABRelease {
			erab, _, err := e.answerDedicatedRelease(ue, enbUEID, frame)
			return erab, err
		}

		if _, err := e.answerModificationFrame(ue, enbUEID, b, frame, nil); err != nil {
			return 0, err
		}
	}
}

func (e *ENB) answerDedicatedRelease(ue *UE, enbUEID int64, frame Frame) (s1ap.ERABID, *eps.DeactivateEPSBearerContextRequest, error) {
	cmd, err := s1ap.ParseERABReleaseCommand(frame.Value)
	if err != nil {
		return 0, nil, fmt.Errorf("parse E-RAB Release Command: %w", err)
	}

	if len(cmd.ERABToBeReleased) != 1 {
		return 0, nil, fmt.Errorf("E-RAB Release Command with %d E-RABs, want 1", len(cmd.ERABToBeReleased))
	}

	if err := e.sendERABReleaseResponse(cmd, enbUEID); err != nil {
		return 0, nil, err
	}

	erab := cmd.ERABToBeReleased[0].ERABID

	if len(cmd.NASPDU) == 0 {
		return erab, nil, nil
	}

	plain, err := ue.unprotectDownlink([]byte(cmd.NASPDU))
	if err != nil {
		return 0, nil, fmt.Errorf("unprotect Deactivate EPS Bearer Context Request: %w", err)
	}

	deact, err := eps.ParseDeactivateEPSBearerContextRequest(plain)
	if err != nil {
		return 0, nil, fmt.Errorf("parse Deactivate EPS Bearer Context Request: %w", err)
	}

	accept, err := ue.buildDeactivateEPSBearerContextAccept(uint8(deact.EPSBearerIdentity), uint8(deact.PTI))
	if err != nil {
		return 0, nil, err
	}

	if err := e.SendUplinkNASTransport(int64(cmd.MMEUES1APID), enbUEID, accept); err != nil {
		return 0, nil, fmt.Errorf("send Deactivate EPS Bearer Context Accept: %w", err)
	}

	return erab, deact, nil
}

func (e *ENB) sendESM(ue *UE, mmeUEID, enbUEID int64, plain []byte) error {
	protected, err := ue.protectUplink(plain)
	if err != nil {
		return err
	}

	return e.SendUplinkNASTransport(mmeUEID, enbUEID, protected)
}

func (e *ENB) AcceptDedicatedModification(ue *UE, enbUEID int64, b *DedicatedBearer, timeout time.Duration) (*eps.ModifyEPSBearerContextRequest, error) {
	return e.answerDedicatedModification(ue, enbUEID, b, timeout, nil)
}

func (e *ENB) RejectDedicatedModification(ue *UE, enbUEID int64, b *DedicatedBearer, cause eps.ESMCause, timeout time.Duration) (*eps.ModifyEPSBearerContextRequest, error) {
	return e.answerDedicatedModification(ue, enbUEID, b, timeout, &cause)
}

func (e *ENB) answerDedicatedModification(ue *UE, enbUEID int64, b *DedicatedBearer, timeout time.Duration, reject *eps.ESMCause) (*eps.ModifyEPSBearerContextRequest, error) {
	deadline := time.Now().Add(timeout)

	for {
		frame, err := e.WaitForAnyMessage(enbUEID, Initiating, []s1ap.ProcedureCode{s1ap.ProcERABModify, s1ap.ProcDownlinkNASTransport}, time.Until(deadline))
		if err != nil {
			return nil, fmt.Errorf("await the modification of E-RAB %d: %w", b.ERABID, err)
		}

		req, err := e.answerModificationFrame(ue, enbUEID, b, frame, reject)
		if err != nil || req != nil {
			return req, err
		}
	}
}

func (e *ENB) answerModificationFrame(ue *UE, enbUEID int64, b *DedicatedBearer, frame Frame, reject *eps.ESMCause) (*eps.ModifyEPSBearerContextRequest, error) {
	naspdu, mmeUEID, err := e.modificationNAS(frame, enbUEID, b)
	if err != nil || naspdu == nil {
		return nil, err
	}

	plain, err := ue.unprotectDownlink(naspdu)
	if err != nil {
		return nil, fmt.Errorf("unprotect downlink NAS: %w", err)
	}

	req, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil || uint8(req.EPSBearerIdentity) != uint8(b.ERABID) {
		return nil, nil
	}

	if reject != nil {
		plain, err := (&eps.ModifyEPSBearerContextReject{EPSBearerIdentity: req.EPSBearerIdentity, PTI: req.PTI, Cause: *reject}).MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("build Modify EPS Bearer Context Reject: %w", err)
		}

		if err := e.sendESM(ue, mmeUEID, enbUEID, plain); err != nil {
			return nil, fmt.Errorf("send Modify EPS Bearer Context Reject: %w", err)
		}

		return req, nil
	}

	if req.NewEPSQoS != nil {
		b.EPSQoS = *req.NewEPSQoS
	}

	if req.TFT != nil {
		b.TFT = applyTFT(b.TFT, *req.TFT)
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

func (e *ENB) modificationNAS(frame Frame, enbUEID int64, b *DedicatedBearer) ([]byte, int64, error) {
	if frame.ProcedureCode == s1ap.ProcDownlinkNASTransport {
		dl, err := s1ap.ParseDownlinkNASTransport(frame.Value)
		if err != nil {
			return nil, 0, fmt.Errorf("parse Downlink NAS Transport: %w", err)
		}

		return []byte(dl.NASPDU), int64(dl.MMEUES1APID), nil
	}

	req, err := s1ap.ParseERABModifyRequest(frame.Value)
	if err != nil {
		return nil, 0, fmt.Errorf("parse E-RAB Modify Request: %w", err)
	}

	for _, item := range req.ERABToBeModified {
		if item.ERABID != b.ERABID {
			continue
		}

		b.QoS = item.QoS

		if err := e.sendERABModifyResponse(req, enbUEID); err != nil {
			return nil, 0, err
		}

		return []byte(item.NASPDU), int64(req.MMEUES1APID), nil
	}

	return nil, 0, fmt.Errorf("E-RAB Modify Request for E-RABs %+v, want %d", req.ERABToBeModified, b.ERABID)
}

func applyTFT(current, change eps.TrafficFlowTemplate) eps.TrafficFlowTemplate {
	out := eps.TrafficFlowTemplate{Operation: eps.TFTCreate}

	switch change.Operation {
	case eps.TFTCreate:
		out.Filters = slices.Clone(change.Filters)
	case eps.TFTAddFilters, eps.TFTReplaceFilters:
		out.Filters = slices.Clone(current.Filters)

		for _, f := range change.Filters {
			out.Filters = slices.DeleteFunc(out.Filters, func(old eps.TFTPacketFilter) bool { return old.Identifier == f.Identifier })
			out.Filters = append(out.Filters, f)
		}
	case eps.TFTDeleteFilters:
		out.Filters = slices.DeleteFunc(slices.Clone(current.Filters), func(old eps.TFTPacketFilter) bool {
			return slices.Contains(change.DeleteIdentifiers, old.Identifier)
		})
	default:
		out.Filters = slices.Clone(current.Filters)
	}

	return out
}
