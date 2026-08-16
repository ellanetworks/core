// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

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

func (a *AMF) AdoptMMContext(ctx context.Context, ue *UeContext, resp interworking.MMContextResponse) error {
	intOrder, encOrder, err := a.SecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("amf: resolve the NAS security algorithms: %w", err)
	}

	if attested := ue.UESecCap(); attested != nil {
		resp.Security.UE5GSecurityCapability = attested
	}

	mapped, err := interworking.MapTo5GSOnIdleMobility(resp.Security, intOrder, encOrder)
	if err != nil {
		return err
	}

	ue.SetSupi(resp.SUPI)
	ue.SetAmbr(&models.Ambr{Uplink: resp.AMBRUplink, Downlink: resp.AMBRDownlink})

	// A UE that has just been served by an MME supports S1 mode, whatever it did
	// or did not manage to say about itself: TS 24.501 §4.4.6 keeps the 5GMM
	// capability IE out of an initial NAS message a UE with no security context
	// can send, so the AMF would otherwise not know until the SECURITY MODE
	// COMPLETE replays it — after the command that has to carry the selected EPS
	// NAS algorithms was already built (§5.4.2.2), leaving the UE unable to
	// derive the same mapped context on its way back.
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, nil)

	if _, ok := ue.EPSNetworkCapability(); !ok {
		if raw, err := resp.UENetworkCapability.MarshalBinary(); err == nil {
			ue.SetUECapabilities(nil, raw)
		}
	}

	return ue.InstallMappedSecurityContextFromEPS(mapped, MintAuthProofForInterworking())
}

func (a *AMF) AckMMContext(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	if a.EPS == nil {
		return ErrNoEPSPeer
	}

	return a.EPS.MMContextAck(ctx, supi, transferred)
}
