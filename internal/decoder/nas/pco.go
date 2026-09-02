// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"

	naslib "github.com/ellanetworks/core/nas"
)

// ProtocolConfigurationOptions is the (extended) protocol configuration options
// element (TS 24.008 §10.5.6.3). The element predates 5GS and both stacks carry
// it, so one rendering serves them.
//
// Containers are listed rather than flattened into named flags: an identifier
// means different things in each direction — 000DH is a DNS server address
// request uplink and the address itself downlink — and the contents belong to
// the protocol the identifier names, so they are kept as bytes.
type ProtocolConfigurationOptions struct {
	ConfigProtocol uint8          `json:"config_protocol"`
	Direction      string         `json:"direction,omitempty"`
	Containers     []PCOContainer `json:"containers,omitempty"`
	Error          string         `json:"error,omitempty"`
}

type PCOContainer struct {
	ID   uint16 `json:"id"`
	Name string `json:"name,omitempty"`
	Hex  string `json:"hex,omitempty"`
}

func pcoDirectionName(d naslib.PCODirection) string {
	switch d {
	case naslib.PCOMSToNetwork:
		return "uplink"
	case naslib.PCONetworkToMS:
		return "downlink"
	default:
		return ""
	}
}

// ExtendedPCO renders the element, or nil when it is absent.
func ExtendedPCO(opts *naslib.ProtocolConfigurationOptions) *ProtocolConfigurationOptions {
	if opts == nil {
		return nil
	}

	out := &ProtocolConfigurationOptions{
		ConfigProtocol: opts.ConfigProtocol,
		Direction:      pcoDirectionName(opts.Direction),
	}

	for _, c := range opts.Containers {
		out.Containers = append(out.Containers, PCOContainer{
			ID:   c.ID,
			Name: naslib.PCOContainerName(c.ID, opts.Direction),
			Hex:  hex.EncodeToString(c.Content),
		})
	}

	return out
}
