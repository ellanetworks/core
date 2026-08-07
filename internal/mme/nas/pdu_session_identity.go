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

func pduSessionIDFromPCO(pco *nas.ProtocolConfigurationOptions) uint8 {
	if pco == nil {
		return 0
	}

	id, ok := pco.PDUSessionID()
	if !ok {
		return 0
	}

	return id
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
