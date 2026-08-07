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
	"math"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
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
	PDUSessionType fgs.PDUSessionType
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
	cause *fgs.GSMCause,
	addrs *PDUSessionAddresses,
	alwaysOn *bool,
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

	var dnnIE *fgs.DNN

	if dnn != "" {
		dnnIE = new(fgs.DNN(dnn))
	}

	m := &fgs.PDUSessionEstablishmentAccept{
		PDUSessionID:        fgs.PDUSessionID(pduSessionID),
		PTI:                 nas.ProcedureTransactionIdentity(pti),
		PDUSessionType:      pduSessionType,
		SSCMode:             fgs.SSCMode(defaultSSCMode),
		QoSRules:            fgs.QoSRules{fgs.DefaultQoSRule(DefaultQosRuleID, qosData.QFI)},
		SessionAMBR:         sessAMBR,
		Cause:               cause,
		SNSSAI:              &fgs.SNSSAI{SST: uint8(snssai.Sst), SD: sd},
		QoSFlowDescriptions: fgs.QoSFlowDescriptions{fgs.FiveQIQoSFlow(qosData.QFI, uint8(qosData.Var5qi), fgs.QoSFlowOpCreate)},
		AlwaysOn:            alwaysOn,
		DNN:                 dnnIE,
	}

	if addrs != nil {
		m.PDUAddress = pduAddress(addrs)
	}

	if pco.DNSIPv4Request || pco.DNSIPv6Request || pco.IPv4LinkMTURequest {
		var dnsServers [][]byte

		if pco.DNSIPv4Request || pco.DNSIPv6Request {
			addr, _ := netip.AddrFromSlice(dns)
			dnsServers = nas.DNSServers(addr)
		}

		// No meaning on a session with no IPv4, matching the PDN-type guard both
		// EPS builders apply. The IPv6 link MTU rides the RA instead (TS 24.501).
		var linkMTU uint16
		if pco.IPv4LinkMTURequest &&
			(pduSessionType == fgs.PDUSessionTypeIPv4 || pduSessionType == fgs.PDUSessionTypeIPv4v6) {
			linkMTU = mtu
		}

		if opts := nas.NewProtocolConfigurationOptions(dnsServers, linkMTU); !opts.Empty() {
			m.ExtendedPCO = &opts
		}
	}

	return m.MarshalBinary()
}

// ModelsToSessionAMBR converts a policy AMBR to the NAS session AMBR
// representation.
func ModelsToSessionAMBR(ambr *models.Ambr) (fgs.SessionAMBR, error) {
	var out fgs.SessionAMBR

	up, upUnit, err := sessionAMBRFields(ambr.Uplink)
	if err != nil {
		return out, fmt.Errorf("uplink session AMBR: %w", err)
	}

	down, downUnit, err := sessionAMBRFields(ambr.Downlink)
	if err != nil {
		return out, fmt.Errorf("downlink session AMBR: %w", err)
	}

	out.Uplink, out.UplinkUnit = up, upUnit
	out.Downlink, out.DownlinkUnit = down, downUnit

	return out, nil
}

// sessionAMBRUnits are the TS 24.501 §9.11.4.14 unit codes, ascending.
var sessionAMBRUnits = []struct {
	multiplier uint64
	code       fgs.SessionAMBRUnit
}{
	{1e3, fgs.SessionAMBRUnit1Kbps},
	{1e6, fgs.SessionAMBRUnit1Mbps},
	{1e9, fgs.SessionAMBRUnit1Gbps},
	{1e12, fgs.SessionAMBRUnit1Tbps},
	{1e15, fgs.SessionAMBRUnit1Pbps},
}

// sessionAMBRFields splits a rate into the value and unit of TS 24.501
// §9.11.4.14. It picks the widest unit that divides the rate evenly and still
// leaves a value inside the field's 16 bits; a rate below 1 Kbps, or one no
// unit divides, has no representation.
func sessionAMBRFields(r models.BitRate) (uint16, fgs.SessionAMBRUnit, error) {
	bps := r.Bps()

	value, unit := uint64(0), fgs.SessionAMBRUnitNotUsed

	for _, u := range sessionAMBRUnits {
		if bps%u.multiplier != 0 {
			continue
		}

		if v := bps / u.multiplier; v >= 1 && v <= math.MaxUint16 {
			value, unit = v, u.code
		}
	}

	if unit == fgs.SessionAMBRUnitNotUsed {
		return 0, unit, fmt.Errorf("%s has no TS 24.501 §9.11.4.14 representation", r)
	}

	return uint16(value), unit, nil
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
