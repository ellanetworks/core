// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

const (
	DefaultQosRuleID uint8 = 1
	defaultSSCMode   uint8 = 1
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

// PDUSessionAddresses holds the address information for a PDU session.
// For IPv4-only, only IPv4Address is set. For IPv6-only, only IPv6IID is set.
// For IPv4v6, both are set.
type PDUSessionAddresses struct {
	PDUSessionType uint8
	IPv4Address    net.IP
	IPv6IID        [8]byte
}

func BuildGSMPDUSessionEstablishmentAccept(
	ambr *models.Ambr,
	qosData *models.QosData,
	pduSessionID uint8,
	pti uint8,
	snssai *models.Snssai,
	dnn string,
	pco *ProtocolConfigurationOptions,
	dns net.IP,
	mtu uint16,
	cause uint8,
	addrs *PDUSessionAddresses,
	alwaysOn *uint8,
) ([]byte, error) {
	pduSessionType := fgs.PDUSessionTypeIPv4
	if addrs != nil {
		pduSessionType = addrs.PDUSessionType
	}

	sessAMBR, err := ModelsToSessionAMBR(ambr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to SessionAMBR: %v", err)
	}

	sd, err := parseSD(snssai.Sd)
	if err != nil {
		return nil, err
	}

	m := &fgs.PDUSessionEstablishmentAccept{
		PDUSessionID:        pduSessionID,
		PTI:                 pti,
		PDUSessionType:      pduSessionType,
		SSCMode:             defaultSSCMode,
		QoSRules:            fgs.MarshalQoSRules([]fgs.QoSRule{fgs.DefaultQoSRule(DefaultQosRuleID, qosData.QFI)}),
		SessionAMBR:         sessAMBR,
		Cause:               cause,
		SNSSAI:              &fgs.SNSSAI{SST: uint8(snssai.Sst), SD: sd},
		QoSFlowDescriptions: fgs.MarshalCreateQoSFlow(qosData.QFI, uint8(qosData.Var5qi)),
		AlwaysOn:            alwaysOn,
		DNN:                 dnn,
	}

	if addrs != nil {
		m.PDUAddress = pduAddress(addrs)
	}

	if pco.DNSIPv4Request || pco.DNSIPv6Request || pco.IPv4LinkMTURequest {
		var opts fgs.PCO

		if pco.DNSIPv4Request {
			opts.AddDNSServerIPv4Address(dns)
		}

		if pco.DNSIPv6Request {
			opts.AddDNSServerIPv6Address(dns)
		}

		if pco.IPv4LinkMTURequest {
			opts.AddIPv4LinkMTU(mtu)
		}

		m.ExtendedPCO = opts.Marshal()
	}

	return m.Marshal()
}

// ModelsToSessionAMBR converts a policy AMBR ("<value> <unit>" strings) to the
// NAS session AMBR representation.
func ModelsToSessionAMBR(ambr *models.Ambr) (fgs.SessionAMBR, error) {
	var out fgs.SessionAMBR

	uplink := strings.Split(ambr.Uplink, " ")

	up, err := strconv.ParseUint(uplink[0], 10, 16)
	if err != nil {
		return out, fmt.Errorf("failed to parse uplink bitrate: %v", err)
	}

	downlink := strings.Split(ambr.Downlink, " ")

	down, err := strconv.ParseUint(downlink[0], 10, 16)
	if err != nil {
		return out, fmt.Errorf("failed to parse downlink bitrate: %v", err)
	}

	out.Uplink = uint16(up)
	out.UplinkUnit = strToAMBRUnit(uplink[1])
	out.Downlink = uint16(down)
	out.DownlinkUnit = strToAMBRUnit(downlink[1])

	return out, nil
}

func strToAMBRUnit(unit string) uint8 {
	switch unit {
	case "Kbps":
		return fgs.SessionAMBRUnit1Kbps
	case "Mbps":
		return fgs.SessionAMBRUnit1Mbps
	case "Gbps":
		return fgs.SessionAMBRUnit1Gbps
	case "Tbps":
		return fgs.SessionAMBRUnit1Tbps
	case "Pbps":
		return fgs.SessionAMBRUnit1Pbps
	}

	return fgs.SessionAMBRUnitNotUsed
}

func parseSD(sd string) (*[3]byte, error) {
	if sd == "" {
		return nil, nil
	}

	b, err := hex.DecodeString(sd)
	if err != nil {
		return nil, fmt.Errorf("failed to decode sd: %v", err)
	}

	var out [3]byte

	copy(out[:], b)

	return &out, nil
}

func pduAddress(addrs *PDUSessionAddresses) *fgs.PDUAddress {
	a := &fgs.PDUAddress{SessionType: addrs.PDUSessionType, IPv6IID: addrs.IPv6IID}
	copy(a.IPv4[:], addrs.IPv4Address.To4())

	return a
}
