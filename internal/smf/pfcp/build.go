// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"net"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

type Flag uint8

// setBit sets the bit at the given position to the specified value (true or false)
// Positions go from 1 to 8
func (f *Flag) setBit(position uint8, value bool) {
	if position < 1 || position > 8 {
		return
	}
	if value {
		*f |= 1 << (position - 1)
	} else {
		*f &= ^(1 << (position - 1))
	}
}

func createPDIIE(pdi *context.PDI) *ie.IE {
	createPDIIes := make([]*ie.IE, 0)
	createPDIIes = append(createPDIIes,
		ie.NewSourceInterface(pdi.SourceInterface.InterfaceValue),
	)

	if pdi.LocalFTeid != nil {
		fteidFlags := new(Flag)
		fteidFlags.setBit(1, pdi.LocalFTeid.V4)
		fteidFlags.setBit(2, pdi.LocalFTeid.V6)
		fteidFlags.setBit(3, pdi.LocalFTeid.Ch)
		fteidFlags.setBit(4, pdi.LocalFTeid.Chid)
		createPDIIes = append(createPDIIes,
			ie.NewFTEID(
				uint8(*fteidFlags),
				pdi.LocalFTeid.Teid,
				pdi.LocalFTeid.Ipv4Address,
				pdi.LocalFTeid.Ipv6Address,
				pdi.LocalFTeid.ChooseId,
			),
		)
	}

	createPDIIes = append(createPDIIes,
		ie.NewNetworkInstance(string(pdi.NetworkInstance)),
	)
	if pdi.UEIPAddress != nil {
		ueIPAddressflags := new(Flag)
		ueIPAddressflags.setBit(1, pdi.UEIPAddress.V6)
		ueIPAddressflags.setBit(2, pdi.UEIPAddress.V4)
		ueIPAddressflags.setBit(3, pdi.UEIPAddress.Sd)
		ueIPAddressflags.setBit(4, pdi.UEIPAddress.Ipv6d)
		ueIPAddressflags.setBit(5, pdi.UEIPAddress.CHV4)
		ueIPAddressflags.setBit(6, pdi.UEIPAddress.CHV6)
		createPDIIes = append(createPDIIes,
			ie.NewUEIPAddress(
				uint8(*ueIPAddressflags),
				pdi.UEIPAddress.Ipv4Address.String(),
				pdi.UEIPAddress.Ipv6Address.String(),
				pdi.UEIPAddress.Ipv6PrefixDelegationBits,
				0,
			),
		)
	}

	if pdi.ApplicationID != "" {
		createPDIIes = append(createPDIIes, ie.NewApplicationID(pdi.ApplicationID))
	}

	if pdi.SDFFilter != nil {
		createPDIIes = append(createPDIIes, ie.NewSDFFilter(
			string(pdi.SDFFilter.FlowDescription),
			string(pdi.SDFFilter.TosTrafficClass),
			string(pdi.SDFFilter.SecurityParameterIndex),
			string(pdi.SDFFilter.FlowLabel),
			0,
		),
		)
	}

	return ie.NewPDI(createPDIIes...)
}

func pdrToCreatePDR(pdr *context.PDR) *ie.IE {
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewPDRID(pdr.PDRID))
	ies = append(ies, ie.NewPrecedence(pdr.Precedence))
	ies = append(ies, createPDIIE(&pdr.PDI))
	if pdr.OuterHeaderRemoval != nil {
		ies = append(ies, ie.NewOuterHeaderRemoval(pdr.OuterHeaderRemoval.OuterHeaderRemovalDescription, 0))
	}
	if pdr.FAR != nil {
		ies = append(ies, ie.NewFARID(pdr.FAR.FARID))
	}
	for _, qer := range pdr.QER {
		if qer != nil {
			ies = append(ies, ie.NewQERID(qer.QERID))
		}
	}
	return ie.NewCreatePDR(ies...)
}

func farToCreateFAR(far *context.FAR) *ie.IE {
	createFARies := make([]*ie.IE, 0)
	createFARies = append(createFARies, ie.NewFARID(far.FARID))
	applyActionflag := new(Flag)
	applyActionflag.setBit(1, far.ApplyAction.Drop)
	applyActionflag.setBit(2, far.ApplyAction.Forw)
	applyActionflag.setBit(3, far.ApplyAction.Buff)
	applyActionflag.setBit(4, far.ApplyAction.Nocp)
	applyActionflag.setBit(5, far.ApplyAction.Dupl)
	createFARies = append(createFARies, ie.NewApplyAction(uint8(*applyActionflag)))
	if far.BAR != nil {
		createFARies = append(createFARies, ie.NewBARID(far.BAR.BARID))
	}
	if far.ForwardingParameters != nil {
		forwardingParametersIEs := make([]*ie.IE, 0)
		forwardingParametersIEs = append(forwardingParametersIEs, ie.NewDestinationInterface(far.ForwardingParameters.DestinationInterface.InterfaceValue))
		forwardingParametersIEs = append(forwardingParametersIEs, ie.NewNetworkInstance(string(far.ForwardingParameters.NetworkInstance)))
		if far.ForwardingParameters.OuterHeaderCreation != nil {
			forwardingParametersIEs = append(forwardingParametersIEs, ie.NewOuterHeaderCreation(
				far.ForwardingParameters.OuterHeaderCreation.OuterHeaderCreationDescription,
				far.ForwardingParameters.OuterHeaderCreation.Teid,
				far.ForwardingParameters.OuterHeaderCreation.Ipv4Address.String(),
				far.ForwardingParameters.OuterHeaderCreation.Ipv6Address.String(),
				far.ForwardingParameters.OuterHeaderCreation.PortNumber,
				0,
				0,
			))
		}

		if far.ForwardingParameters.ForwardingPolicyID != "" {
			forwardingParametersIEs = append(forwardingParametersIEs, ie.NewForwardingPolicy(far.ForwardingParameters.ForwardingPolicyID))
		}
		createFARies = append(createFARies, ie.NewForwardingParameters(forwardingParametersIEs...))
	}
	return ie.NewCreateFAR(createFARies...)
}

func qerToCreateQER(qer *context.QER) *ie.IE {
	createQERies := make([]*ie.IE, 0)
	createQERies = append(createQERies, ie.NewQERID(qer.QERID))
	if qer.GateStatus != nil {
		createQERies = append(createQERies, ie.NewGateStatus(qer.GateStatus.ULGate, qer.GateStatus.DLGate))
	}
	createQERies = append(createQERies, ie.NewQFI(qer.QFI.QFI))
	if qer.MBR != nil {
		createQERies = append(createQERies, ie.NewMBR(qer.MBR.ULMBR, qer.MBR.DLMBR))
	}
	if qer.GBR != nil {
		createQERies = append(createQERies, ie.NewGBR(qer.GBR.ULGBR, qer.GBR.DLGBR))
	}
	return ie.NewCreateQER(createQERies...)
}

func pdrToUpdatePDR(pdr *context.PDR) *ie.IE {
	updatePDRies := make([]*ie.IE, 0)
	updatePDRies = append(updatePDRies, ie.NewPDRID(pdr.PDRID))
	updatePDRies = append(updatePDRies, ie.NewPrecedence(pdr.Precedence))
	updatePDRies = append(updatePDRies, createPDIIE(&pdr.PDI))
	if pdr.OuterHeaderRemoval != nil {
		updatePDRies = append(updatePDRies, ie.NewOuterHeaderRemoval(pdr.OuterHeaderRemoval.OuterHeaderRemovalDescription, 0))
	}
	if pdr.FAR != nil {
		updatePDRies = append(updatePDRies, ie.NewFARID(pdr.FAR.FARID))
	}
	for _, qer := range pdr.QER {
		if qer != nil {
			updatePDRies = append(updatePDRies, ie.NewQERID(qer.QERID))
		}
	}
	return ie.NewUpdatePDR(updatePDRies...)
}

func farToUpdateFAR(far *context.FAR) *ie.IE {
	updateFARies := make([]*ie.IE, 0)
	updateFARies = append(updateFARies, ie.NewFARID(far.FARID))

	if far.BAR != nil {
		updateFARies = append(updateFARies, ie.NewBARID(far.BAR.BARID))
	}

	applyActionflag := new(Flag)
	applyActionflag.setBit(1, far.ApplyAction.Drop)
	applyActionflag.setBit(2, far.ApplyAction.Forw)
	applyActionflag.setBit(3, far.ApplyAction.Buff)
	applyActionflag.setBit(4, far.ApplyAction.Nocp)
	applyActionflag.setBit(5, far.ApplyAction.Dupl)
	updateFARies = append(updateFARies, ie.NewApplyAction(uint8(*applyActionflag)))

	if far.ForwardingParameters != nil {
		forwardingParametersIEs := make([]*ie.IE, 0)
		forwardingParametersIEs = append(forwardingParametersIEs, ie.NewDestinationInterface(far.ForwardingParameters.DestinationInterface.InterfaceValue))
		forwardingParametersIEs = append(forwardingParametersIEs, ie.NewNetworkInstance(string(far.ForwardingParameters.NetworkInstance)))
		if far.ForwardingParameters.OuterHeaderCreation != nil {
			forwardingParametersIEs = append(forwardingParametersIEs, ie.NewOuterHeaderCreation(
				far.ForwardingParameters.OuterHeaderCreation.OuterHeaderCreationDescription,
				far.ForwardingParameters.OuterHeaderCreation.Teid,
				far.ForwardingParameters.OuterHeaderCreation.Ipv4Address.String(),
				far.ForwardingParameters.OuterHeaderCreation.Ipv6Address.String(),
				far.ForwardingParameters.OuterHeaderCreation.PortNumber,
				0,
				0,
			))
		}
		if far.ForwardingParameters.PFCPSMReqFlags != nil {
			pfcpSMReqFlag := new(Flag)
			pfcpSMReqFlag.setBit(1, far.ForwardingParameters.PFCPSMReqFlags.Drobu)
			pfcpSMReqFlag.setBit(2, far.ForwardingParameters.PFCPSMReqFlags.Sndem)
			pfcpSMReqFlag.setBit(3, far.ForwardingParameters.PFCPSMReqFlags.Qaurr)
			forwardingParametersIEs = append(forwardingParametersIEs, ie.NewPFCPSMReqFlags(uint8(*pfcpSMReqFlag)))
			// reset original far sndem flag
			far.ForwardingParameters.PFCPSMReqFlags = nil
		}

		if far.ForwardingParameters.ForwardingPolicyID != "" {
			forwardingParametersIEs = append(forwardingParametersIEs, ie.NewForwardingPolicy(far.ForwardingParameters.ForwardingPolicyID))
		}
		updateFARies = append(updateFARies, ie.NewUpdateForwardingParameters(forwardingParametersIEs...))
	}
	return ie.NewUpdateFAR(updateFARies...)
}

func BuildPfcpSessionEstablishmentRequest(
	sequenceNumber uint32,
	nodeID string,
	fseidIpv4Address net.IP,
	localSeid uint64,
	pdrList []*context.PDR,
	farList []*context.FAR,
	qerList []*context.QER,
) (*message.SessionEstablishmentRequest, error) {
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewNodeIDHeuristic(nodeID))
	ies = append(ies, ie.NewFSEID(localSeid, fseidIpv4Address, nil))

	for _, pdr := range pdrList {
		if pdr.State == context.RULE_INITIAL {
			ies = append(ies, pdrToCreatePDR(pdr))
		}
	}

	for _, far := range farList {
		if far.State == context.RULE_INITIAL {
			ies = append(ies, farToCreateFAR(far))
		}
		far.State = context.RULE_CREATE
	}

	qerMap := make(map[uint32]*context.QER)
	for _, qer := range qerList {
		qerMap[qer.QERID] = qer
	}
	for _, filteredQER := range qerMap {
		if filteredQER.State == context.RULE_INITIAL {
			ies = append(ies, qerToCreateQER(filteredQER))
		}
		filteredQER.State = context.RULE_CREATE
	}

	ies = append(ies, ie.NewPDNType(ie.PDNTypeIPv4))

	return message.NewSessionEstablishmentRequest(
		1,
		0,
		0,
		sequenceNumber,
		0,
		ies...,
	), nil
}

func BuildPfcpSessionModificationRequest(
	sequenceNumber uint32,
	localSEID uint64,
	remoteSEID uint64,
	fseidIPv4Address net.IP,
	pdrList []*context.PDR,
	farList []*context.FAR,
	qerList []*context.QER,
) (*message.SessionModificationRequest, error) {
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewFSEID(localSEID, fseidIPv4Address, nil))

	for _, pdr := range pdrList {
		switch pdr.State {
		case context.RULE_INITIAL:
			ies = append(ies, pdrToCreatePDR(pdr))
		case context.RULE_UPDATE:
			ies = append(ies, pdrToUpdatePDR(pdr))
		case context.RULE_REMOVE:
			ies = append(ies, ie.NewRemovePDR(ie.NewPDRID(pdr.PDRID)))
		}
		pdr.State = context.RULE_CREATE
	}

	for _, far := range farList {
		switch far.State {
		case context.RULE_INITIAL:
			ies = append(ies, farToCreateFAR(far))
		case context.RULE_UPDATE:
			ies = append(ies, farToUpdateFAR(far))
		case context.RULE_REMOVE:
			ies = append(ies, ie.NewRemoveFAR(ie.NewFARID(far.FARID)))
		}
		far.State = context.RULE_CREATE
	}

	for _, qer := range qerList {
		switch qer.State {
		case context.RULE_INITIAL:
			ies = append(ies, qerToCreateQER(qer))
		}
		qer.State = context.RULE_CREATE
	}
	return message.NewSessionModificationRequest(
		0,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	), nil
}

func BuildPfcpSessionDeletionRequest(
	sequenceNumber uint32,
	localSEID uint64,
	remoteSEID uint64,
	fseidIPv4Address net.IP,
) *message.SessionDeletionRequest {
	return message.NewSessionDeletionRequest(
		1,
		0,
		remoteSEID,
		sequenceNumber,
		12,
		ie.NewFSEID(localSEID, fseidIPv4Address, nil),
	)
}
