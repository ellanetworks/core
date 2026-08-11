// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

func (e *ENB) GlobalENBID() s1ap.GlobalENBID {
	return s1ap.GlobalENBID{
		PLMNIdentity: e.plmn,
		ENBID:        e.enbNodeID(),
	}
}

func (e *ENB) SendHandoverRequired(enbUEID, mmeUEID int64, target s1ap.GlobalENBID) error {
	req := &s1ap.HandoverRequired{
		MMEUES1APID:    s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID:    s1ap.ENBUES1APID(enbUEID),
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		Cause:          s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16}), // handover-desirable-for-radio-reason
		TargetID:       s1ap.TargetID{TargeteNBID: s1ap.TargeteNBID{GlobalENBID: target, SelectedTAI: e.tai()}},
		SourceToTarget: s1ap.TransparentContainer{0x00},
	}

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Handover Required: %w", err)
	}

	return e.SendMessage(b, true)
}

func (e *ENB) WaitForHandoverRequest(timeout time.Duration) (*s1ap.HandoverRequest, error) {
	f, err := e.WaitForMessage(NoUEID, Initiating, s1ap.ProcHandoverResourceAllocation, timeout)
	if err != nil {
		return nil, err
	}

	req, err := s1ap.ParseHandoverRequest(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse Handover Request: %w", err)
	}

	return req, nil
}

func (e *ENB) SendHandoverRequestAcknowledge(targetENBUEID, mmeUEID int64, erabID s1ap.ERABID) (dlTEID uint32, err error) {
	addr := e.n3Addr.To4()
	if addr == nil {
		addr = e.n3Addr.To16()
	}

	dlTEID = e.allocTEID()

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(mmeUEID)),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(targetENBUEID)),
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                erabID,
			TransportLayerAddress: s1ap.TransportLayerAddress(addr),
			GTPTEID:               s1ap.GTPTEID(dlTEID),
		}},
		TargetToSource: s1ap.TransparentContainer{0x00},
	}

	b, err := ack.Marshal()
	if err != nil {
		return 0, fmt.Errorf("s1enb: build Handover Request Acknowledge: %w", err)
	}

	if err := e.SendMessage(b, true); err != nil {
		return 0, err
	}

	return dlTEID, nil
}

func (e *ENB) WaitForHandoverCommand(enbUEID int64, timeout time.Duration) (*s1ap.HandoverCommand, error) {
	f, err := e.WaitForMessage(enbUEID, Successful, s1ap.ProcHandoverPreparation, timeout)
	if err != nil {
		return nil, err
	}

	cmd, err := s1ap.ParseHandoverCommand(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse Handover Command: %w", err)
	}

	return cmd, nil
}

func (e *ENB) SendENBStatusTransfer(mmeUEID, enbUEID int64) error {
	st := &s1ap.ENBStatusTransfer{
		MMEUES1APID: s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		Container:   s1ap.StatusTransferContainer{0x00},
	}

	b, err := st.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build eNB Status Transfer: %w", err)
	}

	return e.SendMessage(b, true)
}

func (e *ENB) WaitForMMEStatusTransfer(enbUEID int64, timeout time.Duration) (*s1ap.MMEStatusTransfer, error) {
	f, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcMMEStatusTransfer, timeout)
	if err != nil {
		return nil, err
	}

	mst, err := s1ap.ParseMMEStatusTransfer(f.Value)
	if err != nil {
		return nil, fmt.Errorf("s1enb: parse MME Status Transfer: %w", err)
	}

	return mst, nil
}

func (e *ENB) SendHandoverNotify(enbUEID, mmeUEID int64) error {
	notify := &s1ap.HandoverNotify{
		MMEUES1APID: s1ap.MMEUES1APID(mmeUEID),
		ENBUES1APID: s1ap.ENBUES1APID(enbUEID),
		EUTRANCGI:   s1ap.Ptr(e.eutranCGI()),
		TAI:         s1ap.Ptr(e.tai()),
	}

	b, err := notify.Marshal()
	if err != nil {
		return fmt.Errorf("s1enb: build Handover Notify: %w", err)
	}

	return e.SendMessage(b, true)
}

func (e *ENB) AnchorAddress(tla s1ap.TransportLayerAddress) (string, error) {
	addr, err := e.selectUpfAddr(tla)
	if err != nil {
		return "", err
	}

	return addr.Unmap().String(), nil
}
