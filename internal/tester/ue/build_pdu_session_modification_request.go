// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type PDUSessionModificationRequestOpts struct {
	PDUSessionID uint8
	PTI          uint8

	ReflectiveQoS     bool
	MultiHomedIPv6    bool
	MaxPacketFilters  uint16
	IntegrityMaxRate  bool
	AlwaysOnRequested bool
	RequestDNSServer  bool

	RequestedQoSFlows fgs.QoSFlowDescriptions
}

func BuildPDUSessionModificationRequest(opts *PDUSessionModificationRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionModificationRequestOpts is nil")
	}

	if opts.PTI == 0 || opts.PTI == 0xff {
		return nil, fmt.Errorf("PDU Session Modification Request needs an assigned PTI, got %d", opts.PTI)
	}

	m := &fgs.PDUSessionModificationRequest{
		PDUSessionID:      fgs.PDUSessionID(opts.PDUSessionID),
		PTI:               nas.ProcedureTransactionIdentity(opts.PTI),
		GSMCapability:     &fgs.GSMCapability{RqoS: opts.ReflectiveQoS, MH6PDU: opts.MultiHomedIPv6},
		RequestedQoSFlows: opts.RequestedQoSFlows,
	}

	if opts.MaxPacketFilters != 0 {
		count := opts.MaxPacketFilters
		m.MaxPacketFilters = &count
	}

	if opts.IntegrityMaxRate {
		m.IntegrityProtMaxDataRate = &[2]byte{0xff, 0xff}
	}

	if opts.AlwaysOnRequested {
		requested := true
		m.AlwaysOnRequested = &requested
	}

	if opts.RequestDNSServer {
		pco := nas.NewRequestedProtocolConfigurationOptions(
			nas.PCOContainerDNSServerIPv4Address,
			nas.PCOContainerDNSServerIPv6Address,
		)
		m.ExtendedPCO = &pco
	}

	return m.MarshalBinary()
}

type PDUSessionModificationCompleteOpts struct {
	PDUSessionID uint8
	PTI          uint8
}

func BuildPDUSessionModificationComplete(opts *PDUSessionModificationCompleteOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("PDUSessionModificationCompleteOpts is nil")
	}

	m := &fgs.PDUSessionModificationComplete{
		PDUSessionID: fgs.PDUSessionID(opts.PDUSessionID),
		PTI:          nas.ProcedureTransactionIdentity(opts.PTI),
	}

	return m.MarshalBinary()
}
