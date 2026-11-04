// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/mme"
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

func interworkingPDUSessionID(m *mme.MME, ue *mme.UeContext, apn string, id uint8) uint8 {
	if !ue.FiveGSInterworkingAllowed() || !m.FiveGSInterworkingSubscribed(ue, apn) {
		return 0
	}

	return id
}

func protocolOptionsFromPCOs(pco, epco *nas.ProtocolConfigurationOptions) ([]nas.PCOContainer, bool) {
	for _, opts := range []*nas.ProtocolConfigurationOptions{epco, pco} {
		if opts == nil {
			continue
		}

		return opts.ProtocolOptions(), true
	}

	return nil, false
}

func pcscfRequestFromPCOs(pco, epco *nas.ProtocolConfigurationOptions) (nas.PCSCFRequest, bool) {
	for _, opts := range []*nas.ProtocolConfigurationOptions{epco, pco} {
		if opts == nil {
			continue
		}

		return opts.PCSCFRequest(), true
	}

	return nas.PCSCFRequest{}, false
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

func snssaiPCOContainer(snssai models.Snssai, home *models.Snssai, plmn models.PlmnID) (nas.PCOContainer, error) {
	sd, err := pcoSD(snssai.Sd)
	if err != nil {
		return nas.PCOContainer{}, err
	}

	ie := fgs.SNSSAI{SST: uint8(snssai.Sst), SD: sd}

	if home != nil {
		hplmnSD, err := pcoSD(home.Sd)
		if err != nil {
			return nas.PCOContainer{}, err
		}

		ie.MappedHPLMNSST = new(uint8(home.Sst))
		ie.MappedHPLMNSD = hplmnSD
	}

	value, err := ie.MarshalBinary()
	if err != nil {
		return nas.PCOContainer{}, fmt.Errorf("encode S-NSSAI: %w", err)
	}

	return nas.NewSNSSAIContainer(value, nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc})
}

func pcoSD(sd string) (*[3]byte, error) {
	sd = models.NormalizeSD(sd)
	if sd == "" {
		return nil, nil
	}

	raw, err := hex.DecodeString(sd)
	if err != nil || len(raw) != 3 {
		return nil, fmt.Errorf("S-NSSAI slice differentiator %q is not three octets", sd)
	}

	return &[3]byte{raw[0], raw[1], raw[2]}, nil
}
