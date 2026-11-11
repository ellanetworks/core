// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func pcscfFamilies(req nas.PCSCFRequest, pdnType eps.PDNType) nas.PCSCFRequest {
	return nas.PCSCFRequest{
		IPv4: req.IPv4 && (pdnType == eps.PDNTypeIPv4 || pdnType == eps.PDNTypeIPv4v6),
		IPv6: req.IPv6 && (pdnType == eps.PDNTypeIPv6 || pdnType == eps.PDNTypeIPv4v6),
	}
}

func WithPCSCF(ctx context.Context, pco nas.ProtocolConfigurationOptions, addrs []netip.Addr, req nas.PCSCFRequest, pdnType eps.PDNType, extended bool) nas.ProtocolConfigurationOptions {
	containers := nas.NewPCSCFContainers(addrs, pcscfFamilies(req, pdnType))
	if len(containers) == 0 {
		return pco
	}

	if extended {
		with := pco
		with.Containers = append(slices.Clone(pco.Containers), containers...)

		return with
	}

	with, dropped := pco.AppendFittingUnextended(containers)
	if dropped > 0 {
		logger.From(ctx, logger.MmeLog).Warn("P-CSCF addresses left out of the protocol configuration options to fit 253 octets",
			zap.Int("dropped", dropped), zap.Int("kept", len(containers)-dropped))
	}

	return with
}
