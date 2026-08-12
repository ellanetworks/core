// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.301 §6.5.1.2
func pduSessionIDFromPCOs(pco, epco *nas.ProtocolConfigurationOptions) uint8 {
	for _, opts := range []*nas.ProtocolConfigurationOptions{epco, pco} {
		if opts == nil {
			continue
		}

		if id, ok := opts.PDUSessionID(); ok {
			return id
		}
	}

	return 0
}

func fiveGSMCauseFromPCOs(pco, epco *nas.ProtocolConfigurationOptions) (uint8, bool) {
	for _, opts := range []*nas.ProtocolConfigurationOptions{epco, pco} {
		if opts == nil {
			continue
		}

		if cause, ok := opts.FiveGSMCause(); ok {
			return cause, true
		}
	}

	return 0, false
}

func snssaiPCOContainer(snssai models.Snssai, plmn models.PlmnID) (nas.PCOContainer, error) {
	ie := fgs.SNSSAI{SST: uint8(snssai.Sst)}

	if sd := models.NormalizeSD(snssai.Sd); sd != "" {
		raw, err := hex.DecodeString(sd)
		if err != nil || len(raw) != 3 {
			return nas.PCOContainer{}, fmt.Errorf("S-NSSAI slice differentiator %q is not three octets", snssai.Sd)
		}

		ie.SD = &[3]byte{raw[0], raw[1], raw[2]}
	}

	value, err := ie.MarshalBinary()
	if err != nil {
		return nas.PCOContainer{}, fmt.Errorf("encode S-NSSAI: %w", err)
	}

	return nas.NewSNSSAIContainer(value, nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc})
}
