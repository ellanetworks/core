// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
)

type GTPTunnel struct {
	PDR    *PDR
	TEID   uint32
	N3IPv4 netip.Addr
	N3IPv6 netip.Addr
}

type DataPath struct {
	UpLinkTunnel   *GTPTunnel
	DownLinkTunnel *GTPTunnel
	SecondPDR      *PDR
	Activated      bool

	// allocated* record every id-generator ID allocated for this data path so
	// teardown frees each exactly once. Freeing by walking the PDR structs
	// double-frees FAR/QER/URR IDs shared across PDRs (a second session can then
	// be handed a still-in-use ID) and cannot reach an allocated-but-unattached
	// ID; a ledger keyed by ID avoids both.
	allocatedPDRIDs map[int64]struct{}
	allocatedFARIDs map[int64]struct{}
	allocatedQERIDs map[int64]struct{}
	allocatedURRIDs map[int64]struct{}
}

func (dp *DataPath) ensureLedger() {
	if dp.allocatedPDRIDs == nil {
		dp.allocatedPDRIDs = make(map[int64]struct{})
		dp.allocatedFARIDs = make(map[int64]struct{})
		dp.allocatedQERIDs = make(map[int64]struct{})
		dp.allocatedURRIDs = make(map[int64]struct{})
	}
}

// trackPDR records a PDR's ID and its FAR's ID (NewPDR allocates both).
func (dp *DataPath) trackPDR(pdr *PDR) {
	dp.ensureLedger()
	dp.allocatedPDRIDs[int64(pdr.PDRID)] = struct{}{}

	if pdr.FAR != nil {
		dp.allocatedFARIDs[int64(pdr.FAR.FARID)] = struct{}{}
	}
}

func (dp *DataPath) trackQER(qer *QER) {
	dp.ensureLedger()
	dp.allocatedQERIDs[int64(qer.QERID)] = struct{}{}
}

func (dp *DataPath) trackURR(urr *URR) {
	dp.ensureLedger()
	dp.allocatedURRIDs[int64(urr.URRID)] = struct{}{}
}

func (dp *DataPath) ActivateUpLinkPdr(ueIP netip.Addr, anIP net.IP, defQER *QER, defURR *URR) {
	dp.UpLinkTunnel.PDR.QER = defQER
	dp.UpLinkTunnel.PDR.URR = defURR

	dp.UpLinkTunnel.PDR.PDI.LocalFTEID = &models.FTEID{}
	dp.UpLinkTunnel.PDR.PDI.UEIPAddress = ueIP

	ohr := models.OuterHeaderRemovalGtpUUdpIpv4
	if anIP != nil && anIP.To4() == nil {
		ohr = models.OuterHeaderRemovalGtpUUdpIpv6
	}

	dp.UpLinkTunnel.PDR.OuterHeaderRemoval = &ohr

	dp.UpLinkTunnel.PDR.FAR.ApplyAction = models.ApplyAction{
		Forw: true,
	}
	dp.UpLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{}
}

func (dp *DataPath) ActivateDlLinkPdr(anIPv4 net.IP, anIPv6 net.IP, teid uint32, ueIP netip.Addr, defQER *QER, defURR *URR) {
	dp.DownLinkTunnel.PDR.QER = defQER
	dp.DownLinkTunnel.PDR.URR = defURR

	dp.DownLinkTunnel.PDR.PDI.UEIPAddress = ueIP

	if anIPv6 != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv6,
				TEID:        teid,
				IPv6Address: anIPv6,
			},
		}
	} else if anIPv4 != nil {
		dp.DownLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        teid,
				IPv4Address: anIPv4.To4(),
			},
		}
	}
}

func (dp *DataPath) ActivateTunnelAndPDR(smf *SMF, smContext *SMContext, policy *Policy, ueIP netip.Addr) error {
	seid := smf.AllocateLocalSEID()

	smContext.SetPFCPSession(seid)

	ulPdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("could not create uplink PDR: %s", err)
	}

	dp.UpLinkTunnel.PDR = ulPdr
	dp.trackPDR(ulPdr)

	dlPdr, err := smf.NewPDR()
	if err != nil {
		return fmt.Errorf("could not create downlink PDR: %s", err)
	}

	dp.DownLinkTunnel.PDR = dlPdr
	dp.trackPDR(dlPdr)

	defQER, err := smf.NewQER(policy)
	if err != nil {
		return fmt.Errorf("could not create QER: %v", err)
	}

	dp.trackQER(defQER)

	defULURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("could not create uplink URR: %v", err)
	}

	dp.trackURR(defULURR)

	defDLURR, err := smf.NewURR()
	if err != nil {
		return fmt.Errorf("could not create downlink URR: %v", err)
	}

	dp.trackURR(defDLURR)

	dp.ActivateUpLinkPdr(ueIP, smContext.Tunnel.ANInformation.IPv4Address, defQER, defULURR)

	dp.ActivateDlLinkPdr(smContext.Tunnel.ANInformation.IPv4Address, smContext.Tunnel.ANInformation.IPv6Address, smContext.Tunnel.ANInformation.TEID, ueIP, defQER, defDLURR)

	if smContext.PDUIPV4Address != nil && smContext.PDUIPV6Prefix != nil {
		secondPdr, err := smf.NewPDR()
		if err != nil {
			return fmt.Errorf("could not create second downlink PDR: %s", err)
		}

		// Track before overwriting FAR so the throwaway FAR ID NewPDR allocated
		// is freed on teardown; the shared FAR/QER/URR IDs are already tracked.
		dp.trackPDR(secondPdr)

		secondPdr.FAR = dlPdr.FAR
		secondPdr.QER = defQER
		secondPdr.URR = defDLURR
		secondPdr.PDI.UEIPAddress, _ = netip.AddrFromSlice(smContext.PDUIPV6Prefix.To16())

		dp.SecondPDR = secondPdr
	}

	dp.Activated = true

	return nil
}

// DeactivateTunnelAndPDR frees every id-generator ID allocated by
// ActivateTunnelAndPDR exactly once. Safe to call more than once: the ledger is
// cleared after freeing.
func (dp *DataPath) DeactivateTunnelAndPDR(smf *SMF) {
	for id := range dp.allocatedPDRIDs {
		smf.pdrIDs.FreeID(id)
	}

	for id := range dp.allocatedFARIDs {
		smf.farIDs.FreeID(id)
	}

	for id := range dp.allocatedQERIDs {
		smf.qerIDs.FreeID(id)
	}

	for id := range dp.allocatedURRIDs {
		smf.urrIDs.FreeID(id)
	}

	dp.allocatedPDRIDs = nil
	dp.allocatedFARIDs = nil
	dp.allocatedQERIDs = nil
	dp.allocatedURRIDs = nil

	dp.UpLinkTunnel = &GTPTunnel{}
	dp.DownLinkTunnel = &GTPTunnel{}
	dp.SecondPDR = nil
	dp.Activated = false
}
