// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/ngap"
)

type HandoverRequiredOpts struct {
	AMFUENGAPID                        int64
	RANUENGAPID                        int64
	HandoverType                       ngap.HandoverType
	Cause                              *ngap.Cause
	TargetMcc                          string
	TargetMnc                          string
	TargetGnbID                        string
	TargetTac                          string
	TargetENBID                        *uint32
	PDUSessions                        []HandoverRequiredPDUSession
	SourceToTargetTransparentContainer []byte
}

type HandoverRequiredPDUSession struct {
	PDUSessionID             int64
	HandoverRequiredTransfer []byte
}

func BuildHandoverRequired(opts *HandoverRequiredOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("HandoverRequiredOpts is nil")
	}

	if opts.TargetMcc == "" || opts.TargetMnc == "" || opts.TargetTac == "" {
		return nil, fmt.Errorf("target identity fields are required")
	}

	if opts.TargetENBID == nil && opts.TargetGnbID == "" {
		return nil, fmt.Errorf("target identity fields are required")
	}

	tac, err := TACValue(opts.TargetTac)
	if err != nil {
		return nil, fmt.Errorf("target TAC: %w", err)
	}

	targetID, err := handoverTargetID(opts, tac)
	if err != nil {
		return nil, err
	}

	cause := opts.Cause
	if cause == nil {
		cause = &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverDesirableForRadio}
	}

	sessions := make(ngap.PDUSessionResourceListHORqd, 0, len(opts.PDUSessions))

	for _, ps := range opts.PDUSessions {
		transfer := ngap.TransferContainer(ps.HandoverRequiredTransfer)
		if transfer == nil {
			transfer, err = buildMinimalHandoverRequiredTransfer()
			if err != nil {
				return nil, fmt.Errorf("build HandoverRequiredTransfer for session %d: %w", ps.PDUSessionID, err)
			}
		}

		sessions = append(sessions, ngap.PDUSessionResourceItemHORqd{
			PDUSessionID: ngap.PDUSessionID(ps.PDUSessionID),
			Transfer:     transfer,
		})
	}

	container := opts.SourceToTargetTransparentContainer
	if container == nil {
		container = []byte{0x00}
	}

	msg := &ngap.HandoverRequired{
		AMFUENGAPID:                        ngap.AMFUENGAPID(opts.AMFUENGAPID),
		RANUENGAPID:                        ngap.RANUENGAPID(opts.RANUENGAPID),
		HandoverType:                       opts.HandoverType,
		Cause:                              cause,
		TargetID:                           targetID,
		PDUSessionResourceListHORqd:        sessions,
		SourceToTargetTransparentContainer: container,
	}

	return msg.Marshal()
}

func handoverTargetID(opts *HandoverRequiredOpts, tac ngap.TAC) (ngap.TargetID, error) {
	plmn, err := PLMNIdentity(opts.TargetMcc, opts.TargetMnc)
	if err != nil {
		return ngap.TargetID{}, fmt.Errorf("target PLMN: %w", err)
	}

	if opts.TargetENBID != nil {
		return ngap.TargetID{TargeteNBID: &ngap.TargeteNBID{
			GlobalENBID: ngap.GlobalNgENBID{
				PLMNIdentity: plmn,
				NgENBID:      ngap.NgENBID{Kind: ngap.NgENBIDMacro, Value: *opts.TargetENBID},
			},
			SelectedEPSTAI: ngap.EPSTAI{PLMNIdentity: plmn, TAC: ngap.EPSTAC(tac)},
		}}, nil
	}

	node, err := GNBNodeID(opts.TargetMcc, opts.TargetMnc, opts.TargetGnbID)
	if err != nil {
		return ngap.TargetID{}, fmt.Errorf("target gNB: %w", err)
	}

	return ngap.TargetID{TargetRANNodeID: &ngap.TargetRANNodeID{
		GlobalRANNodeID: node,
		SelectedTAI:     ngap.TAI{PLMNIdentity: node.PLMNIdentity, TAC: tac},
	}}, nil
}

func buildMinimalHandoverRequiredTransfer() (ngap.TransferContainer, error) {
	return (&ngap.HandoverRequiredTransfer{}).Marshal()
}

func BuildHandoverRequiredTransferWithDirectForwarding(_ netip.Addr) ([]byte, error) {
	available := ngap.DirectForwardingPathAvailable

	return (&ngap.HandoverRequiredTransfer{DirectForwardingPathAvailability: &available}).Marshal()
}
