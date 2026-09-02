// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// TAURejectResult reports how the network answered an unresolvable TRACKING AREA
// UPDATE REQUEST.
type TAURejectResult struct {
	EMMCause     eps.EMMCause
	ReleaseCause s1ap.Cause
}

// UnknownGUTI is a GUTI no MME can resolve, as a UE arriving from another core
// presents (TS 24.301 §5.5.3.2.5).
func UnknownGUTI() eps.EPSMobileIdentity {
	return eps.GUTIIdentity(eps.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0xffff, MMECode: 0xff,
		TMSI: [4]byte{0xde, 0xad, 0xbe, 0xef},
	})
}

// BuildPlainTrackingAreaUpdateRequest builds an unprotected TRACKING AREA UPDATE
// REQUEST carrying no key set, as a UE with no security context for the GUTI it
// presents sends (TS 24.301 §8.2.29).
func (ue *UE) BuildPlainTrackingAreaUpdateRequest(guti eps.EPSMobileIdentity) ([]byte, error) {
	b, err := (&eps.TrackingAreaUpdateRequest{
		EPSUpdateType:       eps.EPSUpdateTypeTA,
		NASKeySetIdentifier: nas.NoKeySet,
		OldGUTI:             guti,
		UENetworkCapability: &eps.UENetworkCapability{EEA: ue.netCapEEA, EIA: ue.netCapEIA},
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("s1enb: build Tracking Area Update Request: %w", err)
	}

	return b, nil
}

// TrackingAreaUpdateRejected drives a TRACKING AREA UPDATE the MME cannot
// resolve: the reject and the release of the bare S1 connection that answered it
// (TS 24.301 §5.5.3.2.5).
func (e *ENB) TrackingAreaUpdateRejected(ue *UE, guti eps.EPSMobileIdentity, timeout time.Duration) (*TAURejectResult, error) {
	if guti.GUTI == nil {
		return nil, fmt.Errorf("s1enb: tracking area update requires a GUTI identity")
	}

	tau, err := ue.BuildPlainTrackingAreaUpdateRequest(guti)
	if err != nil {
		return nil, err
	}

	enbUEID := e.AllocateENBUEID()

	if err := e.SendInitialUEMessageWithSTMSI(enbUEID, guti.GUTI.MMECode, binary.BigEndian.Uint32(guti.GUTI.TMSI[:]), tau); err != nil {
		return nil, err
	}

	downlink, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, timeout)
	if err != nil {
		return nil, fmt.Errorf("await Tracking Area Update Reject: %w", err)
	}

	reject, err := expectDownlink[*eps.TrackingAreaUpdateReject](downlink)
	if err != nil {
		return nil, err
	}

	cmd, err := e.WaitForUEContextReleaseCommand(enbUEID, timeout)
	if err != nil {
		return nil, fmt.Errorf("await UE Context Release Command after the reject: %w", err)
	}

	out := &TAURejectResult{EMMCause: reject.Cause}
	if cmd.Cause != nil {
		out.ReleaseCause = *cmd.Cause
	}

	if err := e.SendUEContextReleaseComplete(mmeUEID, enbUEID); err != nil {
		return nil, err
	}

	return out, nil
}
