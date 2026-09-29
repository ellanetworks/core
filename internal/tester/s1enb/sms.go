// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func (ue *UE) RequestSMSOnly() {
	ue.attachType = eps.AttachTypeCombined
	ue.smsOnly = true
}

func (ue *UE) UseConnection(mmeUEID, enbUEID int64) {
	ue.mmeUEID.Store(mmeUEID)
	ue.enbUEID.Store(enbUEID)
}

func (ue *UE) SMSDetached() bool {
	return ue.smsDetached.Load()
}

func (ue *UE) sendSMS(cp []byte) error {
	plain, err := (&eps.UplinkNASTransport{NASMessageContainer: cp}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Uplink NAS Transport: %w", err)
	}

	return ue.sendProtected(plain)
}

func (ue *UE) sendProtected(plain []byte) error {
	ue.nasMu.Lock()
	defer ue.nasMu.Unlock()

	wire, err := ue.protectUplink(plain)
	if err != nil {
		return err
	}

	return ue.enb.SendUplinkNASTransport(ue.mmeUEID.Load(), ue.enbUEID.Load(), wire)
}

func (ue *UE) dispatchUnsolicited(mmeUEID, enbUEID int64, plain []byte) (bool, error) {
	msg, err := eps.ParseMessage(plain, nas.DirectionDownlink)
	if err != nil {
		return false, nil
	}

	switch m := msg.(type) {
	case *eps.DownlinkNASTransport:
		ue.UseConnection(mmeUEID, enbUEID)
		return true, ue.SMS.Deliver(m.NASMessageContainer)
	case *eps.DetachRequestNetwork:
		if m.TypeOfDetach != eps.DetachTypeNetworkIMSI {
			return false, nil
		}

		ue.UseConnection(mmeUEID, enbUEID)
		ue.smsDetached.Store(true)

		accept, err := (&eps.DetachAccept{}).MarshalBinary()
		if err != nil {
			return true, err
		}

		return true, ue.sendProtected(accept)
	}

	return false, nil
}

func (e *ENB) ServeNAS(ue *UE, mmeUEID, enbUEID int64) (stop func()) {
	ue.UseConnection(mmeUEID, enbUEID)

	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)

		for {
			select {
			case <-done:
				return
			default:
			}

			wire, mme, err := e.WaitForDownlinkNAS(enbUEID, 100*time.Millisecond)
			if err != nil {
				continue
			}

			ue.nasMu.Lock()
			plain, err := ue.unprotectDownlink(wire)
			ue.nasMu.Unlock()

			if err != nil {
				continue
			}

			if handled, _ := ue.dispatchUnsolicited(mme, enbUEID, plain); handled {
				continue
			}

			if mt, err := eps.PeekMessageType(plain); err == nil && mt == eps.MsgGUTIReallocationCommand {
				complete, err := (&eps.GUTIReallocationComplete{}).MarshalBinary()
				if err == nil {
					_ = ue.sendProtected(complete)
				}
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}

func (e *ENB) CombinedTrackingAreaUpdate(ue *UE, guti *eps.EPSMobileIdentity, status *nas.EPSBearerContextStatus, timeout time.Duration) (*eps.TrackingAreaUpdateAccept, error) {
	if guti == nil {
		return nil, fmt.Errorf("s1enb: tracking area update requires the UE's GUTI")
	}

	enbUEID := e.AllocateENBUEID()

	tau, err := ue.buildTrackingAreaUpdateRequestWithBearerStatus(eps.EPSUpdateTypeCombinedTALA, false, guti, status)
	if err != nil {
		return nil, err
	}

	if err := e.SendInitialUEMessageWithSTMSI(enbUEID, guti.GUTI.MMECode, binary.BigEndian.Uint32(guti.GUTI.TMSI[:]), tau); err != nil {
		return nil, err
	}

	_, accept, err := e.acceptIdleTrackingAreaUpdate(ue, enbUEID, timeout)

	return accept, err
}

func (e *ENB) SwitchOff(ue *UE, mmeUEID, enbUEID int64, timeout time.Duration) error {
	detach, err := ue.buildDetachRequestWith(true)
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, detach); err != nil {
		return fmt.Errorf("send Detach Request (switch off): %w", err)
	}

	return e.completeContextRelease(enbUEID, timeout)
}
