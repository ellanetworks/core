// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

func (ue *UeContext) TransferableEPSSessions(requested []uint8) []interworking.PDNConnection {
	if !ue.TransfersToEPS() {
		return nil
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	asked := make(map[uint8]struct{}, len(requested))
	for _, pduSessionID := range requested {
		asked[pduSessionID] = struct{}{}
	}

	out := make([]interworking.PDNConnection, 0, len(requested))

	for pduSessionID, sc := range ue.SmContextList {
		if _, ok := asked[pduSessionID]; !ok {
			continue
		}

		ebi := sc.EBI
		if ebi == 0 {
			continue
		}

		if sc.Snssai == nil {
			continue
		}

		snssai := *sc.Snssai

		out = append(out, interworking.PDNConnection{
			PDUSessionID:      pduSessionID,
			EPSBearerIdentity: ebi,
			APN:               sc.Dnn,
			Snssai:            snssai,
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
		SelectedEPSTAI: interworking.EPSTAI{
			PlmnID: util.PLMNToModels(target.SelectedEPSTAI.PLMNIdentity),
			TAC:    uint16(target.SelectedEPSTAI.TAC),
		},
	}, nil
}

func (ue *UeContext) BuildForwardRelocationRequest(target interworking.ENBIdentity, sourceToTarget []byte, requested []uint8, cause *ngap.Cause) (interworking.ForwardRelocationRequest, *interworking.FiveGToEPSHandover, error) {
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
		SUPI:            ue.Supi(),
		SecurityContext: mapped.Context,
		PDNConnections:  sessions,
		Target:          target,
		Cause:           S1APHandoverCause(cause),
		SourceToTarget:  sourceToTarget,
		UEAMBRUplink:    ambr.Uplink,
		UEAMBRDownlink:  ambr.Downlink,
	}, &mapped, nil
}

// TS 29.010 §7.2, TS 29.274 §8.49
func S1APHandoverCause(cause *ngap.Cause) s1ap.Cause {
	if cause != nil && cause.Group == ngap.CauseGroupRadioNetwork {
		switch cause.Value {
		case ngap.CauseRadioNetworkHandoverDesirableForRadio:
			return s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkHandoverDesirableForRadio}
		case ngap.CauseRadioNetworkTimeCriticalHandover:
			return s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkTimeCriticalHandover}
		case ngap.CauseRadioNetworkReduceLoadInServingCell:
			return s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkReduceLoadInServingCell}
		}
	}

	return s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkResourceOptimisationHandover}
}
