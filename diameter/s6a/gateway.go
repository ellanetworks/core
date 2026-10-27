// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"net/netip"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const maxAgentAddrs = 2

type MIP6AgentInfo struct {
	HomeAgentAddresses []netip.Addr
	HomeAgentHost      string
	HomeAgentRealm     string
}

var (
	mip6AgentInfoRules = diameter.Rules{
		ietfKey(diameter.AVPMIPHomeAgentAddress): {Multiple: true},
		ietfKey(diameter.AVPMIPHomeAgentHost):    {},
		ietfKey(diameter.AVPMIP6HomeLinkPrefix):  {},
	}
	mipHomeAgentHostRules = diameter.Rules{
		ietfKey(diameter.AVPDestinationRealm): {},
		ietfKey(diameter.AVPDestinationHost):  {},
	}
)

func (m MIP6AgentInfo) avp() (diameter.AVP, error) {
	switch {
	case len(m.HomeAgentAddresses) == 0 && m.HomeAgentHost == "":
		return diameter.AVP{}, invalidf("MIP6-Agent-Info without an address or host")
	case len(m.HomeAgentAddresses) > maxAgentAddrs:
		return diameter.AVP{}, invalidf("MIP6-Agent-Info with %d addresses", len(m.HomeAgentAddresses))
	case (m.HomeAgentHost == "") != (m.HomeAgentRealm == ""):
		return diameter.AVP{}, invalidf("MIP-Home-Agent-Host needs both a host and a realm")
	}

	var inner []diameter.AVP

	for _, addr := range m.HomeAgentAddresses {
		if !addr.IsValid() || addr.Zone() != "" {
			return diameter.AVP{}, invalidf("MIP-Home-Agent-Address %v", addr)
		}

		inner = append(inner, diameter.Address(diameter.AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, addr))
	}

	if m.HomeAgentHost != "" {
		inner = append(inner, diameter.Grouped(diameter.AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0,
			ietfString(diameter.AVPDestinationRealm, m.HomeAgentRealm),
			ietfString(diameter.AVPDestinationHost, m.HomeAgentHost),
		))
	}

	return diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0, inner...), nil
}

func (d decoder) mip6AgentInfo(a diameter.AVP) (*MIP6AgentInfo, error) {
	inner, err := d.group(a, mip6AgentInfoRules)
	if err != nil {
		return nil, err
	}

	var m MIP6AgentInfo

	addrs := diameter.FindAll(inner, diameter.AVPMIPHomeAgentAddress, 0)
	if len(addrs) > maxAgentAddrs {
		return nil, diameter.WithinGrouped(a, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, addrs[maxAgentAddrs]))
	}

	for _, addrAVP := range addrs {
		addr, err := addrAVP.Address()
		if err != nil {
			return nil, diameter.WithinGrouped(a, tgpp.InvalidAVP(addrAVP))
		}

		m.HomeAgentAddresses = append(m.HomeAgentAddresses, addr)
	}

	if hostAVP, ok := diameter.Find(inner, diameter.AVPMIPHomeAgentHost, 0); ok {
		if m.HomeAgentHost, m.HomeAgentRealm, err = d.mipHomeAgentHost(hostAVP); err != nil {
			return nil, diameter.WithinGrouped(a, err)
		}
	}

	if len(m.HomeAgentAddresses) == 0 && m.HomeAgentHost == "" {
		return nil, tgpp.InvalidAVP(a)
	}

	return &m, nil
}

func (d decoder) mipHomeAgentHost(a diameter.AVP) (string, string, error) {
	host, err := d.group(a, mipHomeAgentHostRules)
	if err != nil {
		return "", "", err
	}

	h, hasHost := diameter.Find(host, diameter.AVPDestinationHost, 0)
	r, hasRealm := diameter.Find(host, diameter.AVPDestinationRealm, 0)

	switch {
	case !hasHost:
		err = tgpp.MissingAVP(diameter.AVPDestinationHost, 0, 0)
	case !hasRealm:
		err = tgpp.MissingAVP(diameter.AVPDestinationRealm, 0, 0)
	case len(h.Data) == 0:
		err = tgpp.InvalidAVP(h)
	case len(r.Data) == 0:
		err = tgpp.InvalidAVP(r)
	}

	if err != nil {
		return "", "", diameter.WithinGrouped(a, err)
	}

	return h.UTF8String(), r.UTF8String(), nil
}
