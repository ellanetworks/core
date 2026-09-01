// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

type UERadioCapabilityInfoIndicationOpts struct {
	AMFUENGAPID       int64
	RANUENGAPID       int64
	UERadioCapability []byte
}

func BuildUERadioCapabilityInfoIndication(opts *UERadioCapabilityInfoIndicationOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UERadioCapabilityInfoIndicationOpts is nil")
	}

	if len(opts.UERadioCapability) == 0 {
		return nil, fmt.Errorf("UE Radio Capability is required to build UERadioCapabilityInfoIndication")
	}

	msg := &ngap.UERadioCapabilityInfoIndication{
		AMFUENGAPID:       ngap.AMFUENGAPID(opts.AMFUENGAPID),
		RANUENGAPID:       ngap.RANUENGAPID(opts.RANUENGAPID),
		UERadioCapability: ngap.UERadioCapability(opts.UERadioCapability),
	}

	return msg.Marshal()
}
