// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"
	"github.com/ellanetworks/core/internal/decoder/rrc"

	"github.com/ellanetworks/core/s1ap"
)

func buildUECapabilityInfoIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUECapabilityInfoIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Capability Info Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDUERadioCapability, s1ap.CriticalityIgnore, rrc.DescribeS1AP(m.UERadioCapability)),
	}

	if len(m.UERadioCapabilityForPaging) > 0 {
		ies = append(ies, ie(s1ap.IDUERadioCapabilityForPaging, s1ap.CriticalityIgnore, hex.EncodeToString(m.UERadioCapabilityForPaging)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Capability Info Indication (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}
