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

// pduSessionIDFromPCO reads the 5GS PDU session identity a UE allocated for the
// PDN connection out of the uplink Protocol Configuration Options (container
// 001AH, TS 24.008 §10.5.6.3). A UE supporting both accesses in
// single-registration mode sends it so the anchor can recognise the connection
// as a PDU session when it moves to 5GS (TS 23.501 §5.17.2.1); the network has
// no other source for it, and a connection without one is simply not
// transferable (TS 23.502 §4.11.1.1 NOTE 5).
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

// snssaiPCOContainer renders the slice the anchor holds the PDN connection on as
// the network-to-MS S-NSSAI container: the S-NSSAI information element's value
// part (TS 24.501 §9.11.2.8) followed by the PLMN identity it relates to. The SD
// is optional, which selects the 1- or 4-octet form of the value part.
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
