// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
	"go.uber.org/zap"
)

const flagPresentIPv4 = 2

func applyPDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) {
	if spdrInfo.Ipv4 != nil {
		if err := bpfObjects.PutPdrDownlink(spdrInfo.Ipv4, spdrInfo.PdrInfo); err != nil {
			logger.UpfLog.Info("Can't apply IPv4 PDR", zap.Error(err))
		}
	} else if spdrInfo.Ipv6 != nil {
		if err := bpfObjects.PutDownlinkPdrIP6(spdrInfo.Ipv6, spdrInfo.PdrInfo); err != nil {
			logger.UpfLog.Info("Can't apply IPv6 PDR", zap.Error(err))
		}
	} else {
		if err := bpfObjects.PutPdrUplink(spdrInfo.TeID, spdrInfo.PdrInfo); err != nil {
			logger.UpfLog.Info("Can't apply GTP PDR", zap.Error(err))
		}
	}
}

func processCreatedPDRs(createdPDRs []SPDRInfo, n3Address net.IP) []*ie.IE {
	var additionalIEs []*ie.IE
	for _, pdr := range createdPDRs {
		if pdr.Allocated {
			if pdr.Ipv4 != nil {
				additionalIEs = append(additionalIEs, ie.NewCreatedPDR(ie.NewPDRID(uint16(pdr.PdrID)), ie.NewUEIPAddress(flagPresentIPv4, pdr.Ipv4.String(), "", 0, 0)))
			} else if pdr.Ipv6 != nil {
			} else {
				additionalIEs = append(additionalIEs, ie.NewCreatedPDR(ie.NewPDRID(uint16(pdr.PdrID)), ie.NewFTEID(0x01, pdr.TeID, cloneIP(n3Address), nil, 0)))
			}
		}
	}
	return additionalIEs
}
