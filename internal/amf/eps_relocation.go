// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/ngap"
)

func (ue *UeContext) TransferableEPSSessions(requested []uint8) []interworking.PDNConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	var asked map[uint8]struct{}

	if requested != nil {
		asked = make(map[uint8]struct{}, len(requested))
		for _, pduSessionID := range requested {
			asked[pduSessionID] = struct{}{}
		}
	}

	out := make([]interworking.PDNConnection, 0, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
		if asked != nil {
			if _, ok := asked[pduSessionID]; !ok {
				continue
			}
		}

		ebi, ok := ue.epsBearerIdentities[pduSessionID]
		if !ok {
			continue
		}

		out = append(out, interworking.PDNConnection{
			PDUSessionID:      pduSessionID,
			EPSBearerIdentity: ebi,
			APN:               sc.Dnn,
			Snssai:            sc.Snssai,
		})
	}

	return out
}

func ENBIdentityFromNGAP(target ngap.TargeteNBID) (interworking.ENBIdentity, error) {
	bits, ok := map[ngap.NgENBIDKind]uint8{
		ngap.NgENBIDMacro:      20,
		ngap.NgENBIDShortMacro: 18,
		ngap.NgENBIDLongMacro:  21,
	}[target.GlobalENBID.NgENBID.Kind]
	if !ok {
		return interworking.ENBIdentity{}, fmt.Errorf("amf: unknown ng-eNB identity kind %d", target.GlobalENBID.NgENBID.Kind)
	}

	return interworking.ENBIdentity{
		PlmnID: util.PLMNToModels(target.GlobalENBID.PLMNIdentity),
		ID:     target.GlobalENBID.NgENBID.Value,
		Bits:   bits,
		EPSTAC: uint16(target.SelectedEPSTAI.TAC),
	}, nil
}

func (ue *UeContext) BuildForwardRelocationRequest(target interworking.ENBIdentity, sourceToTarget []byte, requested []uint8) (interworking.ForwardRelocationRequest, *interworking.FiveGToEPSHandover, error) {
	sessions := ue.TransferableEPSSessions(requested)
	if len(sessions) == 0 {
		return interworking.ForwardRelocationRequest{}, nil, ErrNoTransferableSessions
	}

	mapped, err := ue.MapSecurityContextToEPS()
	if err != nil {
		return interworking.ForwardRelocationRequest{}, nil, err
	}

	ambr := ue.Ambr
	if ambr == nil {
		return interworking.ForwardRelocationRequest{}, nil, fmt.Errorf("amf: UE has no AMBR")
	}

	return interworking.ForwardRelocationRequest{
		IMSI:            interworking.SUPIToIMSI(ue.Supi()),
		SecurityContext: mapped.Context,
		PDNConnections:  sessions,
		Target:          target,
		SourceToTarget:  sourceToTarget,
		UEAMBRUplink:    ambr.Uplink,
		UEAMBRDownlink:  ambr.Downlink,
	}, &mapped, nil
}
