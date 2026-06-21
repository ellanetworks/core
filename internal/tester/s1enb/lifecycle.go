// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// Detach performs a UE-originating EPS detach for an attached UE (TS 24.301
// §5.5.2.2): it sends a DETACH REQUEST, verifies the MME's DETACH ACCEPT, then
// completes the S1 context release the MME triggers.
func (e *ENB) Detach(ue *UE, mmeUEID, enbUEID int64, timeout time.Duration) error {
	detach, err := ue.buildDetachRequest()
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, detach); err != nil {
		return fmt.Errorf("send Detach Request: %w", err)
	}

	// Wait for DETACH ACCEPT, skipping any proactive downlink NAS the MME may have
	// sent after attach (e.g. EMM INFORMATION with the network name/time), which a
	// real UE ignores.
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("await Detach Accept: timed out")
		}

		wire, _, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return fmt.Errorf("await Detach Accept: %w", err)
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		mt, err := eps.PeekMessageType(plain)
		if err != nil {
			return fmt.Errorf("peek downlink NAS: %w", err)
		}

		if mt == eps.MsgDetachAccept {
			break
		}
	}

	return e.completeContextRelease(enbUEID, timeout)
}

// ReleaseContext performs an eNB-initiated S1 UE context release (TS 36.413
// §8.3.2): it sends a UE CONTEXT RELEASE REQUEST and completes the release the
// MME commands. The UE drops to ECM-IDLE.
func (e *ENB) ReleaseContext(mmeUEID, enbUEID int64, cause s1ap.Cause, timeout time.Duration) error {
	if err := e.SendUEContextReleaseRequest(mmeUEID, enbUEID, cause); err != nil {
		return err
	}

	return e.completeContextRelease(enbUEID, timeout)
}

// ServiceRequestResult reports the S1 identifiers and re-established S1-U endpoint
// from a completed service request.
type ServiceRequestResult struct {
	MMEUES1APID int64
	ENBUES1APID int64
	UpfAddress  string // S-GW/UPF S1-U address (uplink target)
	ULTEID      uint32 // S-GW/UPF uplink TEID
	DLTEID      uint32 // eNB downlink TEID reported to the MME
}

// ServiceRequest performs a mobile-originated EPS service request for a UE in
// ECM-IDLE (TS 24.301 §5.6.1): it sends a SERVICE REQUEST identified by the UE's
// S-TMSI and completes the Initial Context Setup the MME uses to re-establish the
// bearer.
func (e *ENB) ServiceRequest(ue *UE, guti *eps.EPSMobileIdentity, timeout time.Duration) (*ServiceRequestResult, error) {
	if guti == nil {
		return nil, fmt.Errorf("s1enb: service request requires the UE's GUTI")
	}

	enbUEID := e.AllocateENBUEID()

	sr, err := ue.buildServiceRequest()
	if err != nil {
		return nil, err
	}

	if err := e.SendInitialUEMessageWithSTMSI(enbUEID, guti.MMECode, guti.MTMSI, sr); err != nil {
		return nil, err
	}

	icsFrame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcInitialContextSetup, timeout)
	if err != nil {
		return nil, fmt.Errorf("await Initial Context Setup Request (service-request re-establishment): %w", err)
	}

	ics, err := s1ap.ParseInitialContextSetupRequest(icsFrame.Value)
	if err != nil {
		return nil, fmt.Errorf("parse Initial Context Setup Request: %w", err)
	}

	if len(ics.ERABToBeSetup) == 0 {
		return nil, fmt.Errorf("service-request Initial Context Setup without an E-RAB")
	}

	erab := ics.ERABToBeSetup[0]

	upf, err := e.selectUpfAddr(erab.TransportLayerAddress)
	if err != nil {
		return nil, err
	}

	dlTEID := e.allocTEID()

	if err := e.sendInitialContextSetupResponse(ics, enbUEID, dlTEID); err != nil {
		return nil, err
	}

	return &ServiceRequestResult{
		MMEUES1APID: int64(ics.MMEUES1APID),
		ENBUES1APID: enbUEID,
		UpfAddress:  upf.Unmap().String(),
		ULTEID:      uint32(erab.GTPTEID),
		DLTEID:      dlTEID,
	}, nil
}

// PeriodicTrackingAreaUpdate performs a mobile-originated periodic TAU for a UE
// in ECM-IDLE (TS 24.301 §5.5.3.3): it sends a periodic TRACKING AREA UPDATE
// REQUEST identified by the UE's S-TMSI, acknowledges the GUTI-reallocating TAU
// ACCEPT with a TAU COMPLETE, and completes the S1 release back to ECM-IDLE.
func (e *ENB) PeriodicTrackingAreaUpdate(ue *UE, guti *eps.EPSMobileIdentity, timeout time.Duration) error {
	if guti == nil {
		return fmt.Errorf("s1enb: tracking area update requires the UE's GUTI")
	}

	enbUEID := e.AllocateENBUEID()

	tau, err := ue.buildTrackingAreaUpdateRequest(eps.EPSUpdateTypePeriodic, false)
	if err != nil {
		return err
	}

	if err := e.SendInitialUEMessageWithSTMSI(enbUEID, guti.MMECode, guti.MTMSI, tau); err != nil {
		return err
	}

	// Await the TAU Accept, skipping any proactive downlink NAS the MME sends on
	// the re-established connection (e.g. EMM INFORMATION 0x61), which a real UE
	// ignores. mmeUEID is taken from the Accept so the Complete is delivered on
	// the connection the MME re-keyed for this update.
	mmeUEID, err := e.awaitDownlinkNAS(ue, enbUEID, eps.MsgTrackingAreaUpdateAccept, timeout)
	if err != nil {
		return fmt.Errorf("await Tracking Area Update Accept: %w", err)
	}

	complete, err := ue.buildTrackingAreaUpdateComplete()
	if err != nil {
		return err
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, complete); err != nil {
		return err
	}

	// With no active flag the MME releases the UE back to ECM-IDLE once the GUTI
	// reallocation is acknowledged (TS 24.301 §5.5.3.2.4).
	return e.completeContextRelease(enbUEID, timeout)
}

// awaitDownlinkNAS waits for a protected downlink NAS message of the wanted EMM
// message type, skipping any proactive messages the MME interleaves (e.g. EMM
// INFORMATION 0x61), which a real UE ignores. It returns the MME-UE-S1AP-ID the
// matching message arrived on.
func (e *ENB) awaitDownlinkNAS(ue *UE, enbUEID int64, want eps.MessageType, timeout time.Duration) (int64, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, fmt.Errorf("timed out awaiting message type %#x", want)
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return 0, err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return 0, fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		mt, err := eps.PeekMessageType(plain)
		if err != nil {
			return 0, fmt.Errorf("peek downlink NAS: %w", err)
		}

		if mt == want {
			return mmeUEID, nil
		}
	}
}

// completeContextRelease awaits the UE CONTEXT RELEASE COMMAND and acknowledges
// it with a UE CONTEXT RELEASE COMPLETE, ending the release procedure.
func (e *ENB) completeContextRelease(enbUEID int64, timeout time.Duration) error {
	cmd, err := e.WaitForUEContextReleaseCommand(enbUEID, timeout)
	if err != nil {
		return fmt.Errorf("await UE Context Release Command: %w", err)
	}

	return e.SendUEContextReleaseComplete(int64(cmd.UES1APIDs.MMEUES1APID), int64(cmd.UES1APIDs.ENBUES1APID))
}
