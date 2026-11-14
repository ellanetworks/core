// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

type AdditionalGUTI struct {
	GUMMEI GUMMEI `json:"gummei"`
	MTMSI  uint32 `json:"m_tmsi"`
}

func buildRerouteNASRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseRerouteNASRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Reroute NAS Request: %v", err)}, ""
	}

	ies := []IE{ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID))}

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	ies = append(ies,
		ie(s1ap.IDS1Message, s1ap.CriticalityReject, rawIEValue{Hex: hex.EncodeToString(m.S1Message)}),
		ie(s1ap.IDMMEGroupID, s1ap.CriticalityReject, uint16(m.MMEGroupID[0])<<8|uint16(m.MMEGroupID[1])),
	)

	if m.AdditionalGUTI != nil {
		ies = append(ies, ie(s1ap.IDAdditionalGUTI, s1ap.CriticalityIgnore, AdditionalGUTI{GUMMEI: gummei(m.AdditionalGUTI.GUMMEI), MTMSI: uint32(m.AdditionalGUTI.MTMSI)}))
	}

	if m.UEUsageType != nil {
		ies = append(ies, ie(s1ap.IDUEUsageType, s1ap.CriticalityIgnore, *m.UEUsageType))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Reroute NAS Request (eNB UE %d)", uint32(m.ENBUES1APID))
}
