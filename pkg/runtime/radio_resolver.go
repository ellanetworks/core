// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"net/netip"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/upf"
)

type radioResolver struct {
	addresses func() map[netip.Addr]string
	names     func() map[string]string
}

func newRadioResolver(smfInstance *smf.SMF, amfInstance *amf.AMF, mmeInstance *mme.MME) *radioResolver {
	return &radioResolver{
		addresses: smfInstance.RadioIDsByAddress,
		names: func() map[string]string {
			out := make(map[string]string)

			for _, r := range amfInstance.ListRadios() {
				out[r.Ref] = r.Name
			}

			for _, r := range mmeInstance.ListRadios() {
				out[r.Ref] = r.Name
			}

			return out
		},
	}
}

func (r *radioResolver) RadiosByAddress() map[netip.Addr]upf.Radio {
	names := r.names()
	addresses := r.addresses()

	out := make(map[netip.Addr]upf.Radio, len(addresses))

	for addr, id := range addresses {
		out[addr] = upf.Radio{ID: id, Name: names[id]}
	}

	return out
}
