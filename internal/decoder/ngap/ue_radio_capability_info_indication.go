// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"github.com/ellanetworks/core/internal/decoder/rrc"

	"github.com/ellanetworks/core/ngap"
)

// UE Radio Capability Info Indication carries the UE's radio capabilities for
// the AMF to store and hand to a target on handover (TS 38.413 §9.2.7.1). The
// container is opaque: its contents are RRC, not NGAP.
func buildUERadioCapabilityInfoIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParseUERadioCapabilityInfoIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse UE Radio Capability Info Indication: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDUERadioCapability, ngap.CriticalityIgnore, rrc.DescribeNGAP(m.UERadioCapability)),
	}

	if m.UERadioCapabilityForPaging != nil {
		ies = append(ies, ie(ngap.IDUERadioCapabilityForPaging, ngap.CriticalityIgnore,
			ueRadioCapabilityForPaging(*m.UERadioCapabilityForPaging)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
