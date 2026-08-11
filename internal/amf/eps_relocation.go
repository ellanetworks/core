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

// TransferableEPSSessions returns the PDU sessions that can become PDN
// connections in EPS: those holding an EPS bearer identity
// (TS 23.502 §4.11.1.2.1 step 2). A session without one is left behind and
// released separately, because the UE would locally release it on arrival
// anyway (TS 24.501 §6.1.4.1).
func (ue *UeContext) TransferableEPSSessions() []interworking.PDNConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]interworking.PDNConnection, 0, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
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

// ENBIdentityFromNGAP converts the Target ID of an NGAP Handover Required naming
// an eNB into the neutral identity N26 carries (TS 38.413 §9.3.1.8).
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

// BuildForwardRelocationRequest assembles what the MME needs to prepare a
// handover of this UE to EPS (TS 23.502 §4.11.1.2.1 step 3).
//
// It maps the security context, which consumes a downlink NAS COUNT, so it is
// called once per handover attempt.
func (ue *UeContext) BuildForwardRelocationRequest(target interworking.ENBIdentity, sourceToTarget []byte) (interworking.ForwardRelocationRequest, *interworking.FiveGToEPSHandover, error) {
	sessions := ue.TransferableEPSSessions()
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
