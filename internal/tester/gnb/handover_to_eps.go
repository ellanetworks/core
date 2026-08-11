// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

// HandoverToEPSOpts names the eNB a source gNB is handing a UE to.
type HandoverToEPSOpts struct {
	AMFUENGAPID int64
	RANUENGAPID int64

	TargetMcc string
	TargetMnc string
	TargetTac string
	// TargetENBID is the eNB's 20-bit macro identity.
	TargetENBID uint32

	PDUSessionIDs []int64
}

// HandoverToEPSCommand is what the AMF answered a handover to EPS with: the
// container the target eNB produced, and the NAS parameters the UE needs to
// build its mapped EPS security context (TS 33.501 §8.3.2 step 8).
type HandoverToEPSCommand struct {
	TargetToSource []byte

	// DownlinkNASCountSequenceNumber is the 8 least significant bits of the
	// downlink NAS COUNT the AMF derived K'ASME from.
	DownlinkNASCountSequenceNumber uint8

	// ReleasedPDUSessions are the sessions the target did not take over, which
	// the source releases toward the UE (TS 23.502 §4.11.1.2.1 step 12).
	ReleasedPDUSessions []int64
}

// SendHandoverRequiredToEPS asks the AMF to hand this UE to an eNB
// (TS 23.502 §4.11.1.2.1 step 1).
func (g *GnodeB) SendHandoverRequiredToEPS(opts *HandoverToEPSOpts) error {
	sessions := make([]HandoverRequiredPDUSession, 0, len(opts.PDUSessionIDs))
	for _, id := range opts.PDUSessionIDs {
		sessions = append(sessions, HandoverRequiredPDUSession{PDUSessionID: id})
	}

	target := opts.TargetENBID

	return g.SendHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID:  opts.AMFUENGAPID,
		RANUENGAPID:  opts.RANUENGAPID,
		HandoverType: ngap.HandoverTypeFiveGSToEPS,
		TargetMcc:    opts.TargetMcc,
		TargetMnc:    opts.TargetMnc,
		TargetTac:    opts.TargetTac,
		TargetENBID:  &target,
		PDUSessions:  sessions,
	})
}

// WaitForHandoverToEPSCommand waits for the AMF's HANDOVER COMMAND and reads out
// what the UE and the source need from it.
func (g *GnodeB) WaitForHandoverToEPSCommand(timeout time.Duration) (*HandoverToEPSCommand, error) {
	frame, err := g.WaitForMessage(Successful, ngap.ProcHandoverPreparation, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Handover Command: %w", err)
	}

	cmd, err := ngap.ParseHandoverCommand(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse Handover Command: %w", err)
	}

	if cmd.HandoverType != ngap.HandoverTypeFiveGSToEPS {
		return nil, fmt.Errorf("gnb: Handover Command handover type = %d, want fivegs-to-eps", cmd.HandoverType)
	}

	// Mandatory when the UE leaves 5GS (TS 38.413 §9.2.3.2, condition
	// iftoEPSUTRA): without it the UE cannot rebuild the downlink NAS COUNT that
	// seeded K'ASME, so the handover would fail on the EPS side with no diagnosis.
	container, err := fgs.ParseN1ModeToS1ModeNASTransparentContainer(cmd.NASSecurityParametersFromNGRAN)
	if err != nil {
		return nil, fmt.Errorf("gnb: NAS security parameters from NG-RAN: %w", err)
	}

	released := make([]int64, 0, len(cmd.PDUSessionResourceToReleaseList))
	for _, item := range cmd.PDUSessionResourceToReleaseList {
		released = append(released, int64(item.PDUSessionID))
	}

	return &HandoverToEPSCommand{
		TargetToSource:                 cmd.TargetToSourceTransparentContainer,
		DownlinkNASCountSequenceNumber: container.SequenceNumber,
		ReleasedPDUSessions:            released,
	}, nil
}
