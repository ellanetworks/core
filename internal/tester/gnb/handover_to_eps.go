// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

type HandoverToEPSOpts struct {
	AMFUENGAPID   int64
	RANUENGAPID   int64
	TargetMcc     string
	TargetMnc     string
	TargetTac     string
	TargetENBID   uint32
	PDUSessionIDs []int64
}

type HandoverToEPSCommand struct {
	TargetToSource                 []byte
	DownlinkNASCountSequenceNumber uint8
	ReleasedPDUSessions            []int64
}

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

func (g *GnodeB) WaitForHandoverPreparationFailure(timeout time.Duration) (*ngap.HandoverPreparationFailure, error) {
	frame, err := g.WaitForMessage(Unsuccessful, ngap.ProcHandoverPreparation, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Handover Preparation Failure: %w", err)
	}

	fail, err := ngap.ParseHandoverPreparationFailure(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse Handover Preparation Failure: %w", err)
	}

	return fail, nil
}
