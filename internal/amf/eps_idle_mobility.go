// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
)

// FetchMMContext asks the MME for the UE's EPS context on an idle-mode
// inter-system change, presenting the 4G-GUTI the UE's 5G-GUTI maps to and the
// TRACKING AREA UPDATE REQUEST it enclosed (TS 23.502 §4.11.1.3.3 step 5a).
//
// The MME verifies the enclosed message against the EPS security context, so a
// context comes back only for a UE that holds the keys: this is what
// authenticates the move (TS 33.501 §8.2).
func (a *AMF) FetchMMContext(ctx context.Context, presented etsi.GUTI5G, epsNAS []byte) (interworking.MMContextResponse, error) {
	if a.EPS == nil {
		return interworking.MMContextResponse{}, ErrNoEPSPeer
	}

	if len(epsNAS) == 0 {
		return interworking.MMContextResponse{}, fmt.Errorf("%w: the registration carried no EPS NAS message container", interworking.ErrIntegrityCheckFailed)
	}

	mapped, err := presented.MappedEPSGUTI()
	if err != nil {
		return interworking.MMContextResponse{}, fmt.Errorf("%w: %w", interworking.ErrUnknownUEContext, err)
	}

	return a.EPS.MMContext(ctx, interworking.MMContextRequest{MappedEPSGUTI: mapped, EPSNAS: epsNAS})
}

// AdoptMMContext takes the EPS context the MME returned onto ue: the identity it
// verified, the UE's subscribed AMBR, and a 5G security context mapped from the
// EPS one (TS 33.501 §8.6.2). The mapped context is activated by the NAS SMC
// that follows, which is why it is installed before the authentication decision
// rather than after (§8.2).
func (a *AMF) AdoptMMContext(ctx context.Context, ue *UeContext, resp interworking.MMContextResponse) error {
	intOrder, encOrder, err := a.SecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("amf: resolve the NAS security algorithms: %w", err)
	}

	mapped, err := interworking.MapTo5GSOnIdleMobility(resp.Security, intOrder, encOrder)
	if err != nil {
		return err
	}

	ue.SetSupi(resp.SUPI)
	ue.Ambr = &models.Ambr{Uplink: resp.AMBRUplink, Downlink: resp.AMBRDownlink}

	// A UE that omitted the S1 UE network capability still has to be able to
	// return to EPS, and the MME's copy is the one it registered under
	// (TS 24.301 §9.9.3.34).
	if _, ok := ue.EPSNetworkCapability(); !ok {
		if raw, err := resp.UENetworkCapability.MarshalBinary(); err == nil {
			ue.SetUECapabilities(nil, raw)
		}
	}

	return ue.InstallMappedSecurityContextFromEPS(mapped, MintAuthProofForInterworking())
}

// AckMMContext tells the MME which PDU sessions 5GS adopted, releasing the UE
// from EPS (TS 23.502 §4.11.1.3.3 step 8). It is sent only once the AMF has
// committed to serving the UE, so an abandoned move leaves the EPS registration
// intact.
func (a *AMF) AckMMContext(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	if a.EPS == nil {
		return ErrNoEPSPeer
	}

	return a.EPS.MMContextAck(ctx, supi, transferred)
}
