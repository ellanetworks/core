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
	asked := make(map[uint8]struct{}, len(requested))
	for _, pduSessionID := range requested {
		asked[pduSessionID] = struct{}{}
	}

	return ue.transferableEPSSessions(func(pduSessionID uint8) bool {
		_, ok := asked[pduSessionID]

		return ok
	})
}

func (ue *UeContext) AllTransferableEPSSessions() []interworking.PDNConnection {
	return ue.transferableEPSSessions(func(uint8) bool { return true })
}

func (ue *UeContext) transferableEPSSessions(include func(pduSessionID uint8) bool) []interworking.PDNConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]interworking.PDNConnection, 0, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
		if !include(pduSessionID) {
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
	id, bits, ok := target.GlobalENBID.NgENBID.ID()
	if !ok {
		return interworking.ENBIdentity{}, fmt.Errorf("amf: unsupported ng-eNB identity alternative")
	}

	plmn, err := util.PLMNToModels(target.GlobalENBID.PLMNIdentity)
	if err != nil {
		return interworking.ENBIdentity{}, fmt.Errorf("amf: target ng-eNB: %w", err)
	}

	taiPlmn, err := util.PLMNToModels(target.SelectedEPSTAI.PLMNIdentity)
	if err != nil {
		return interworking.ENBIdentity{}, fmt.Errorf("amf: selected EPS TAI: %w", err)
	}

	return interworking.ENBIdentity{
		PlmnID: plmn,
		ID:     id,
		Bits:   uint8(bits),
		SelectedEPSTAI: interworking.EPSTAI{
			PlmnID: taiPlmn,
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

	ambr := ue.Ambr()
	if ambr == nil {
		return interworking.ForwardRelocationRequest{}, nil, fmt.Errorf("amf: UE has no AMBR")
	}

	req := interworking.ForwardRelocationRequest{
		SUPI:            ue.Supi(),
		SecurityContext: mapped.Context,
		PDNConnections:  sessions,
		Target:          target,
		Cause:           S1APHandoverCause(cause),
		SourceToTarget:  sourceToTarget,
		UEAMBRUplink:    ambr.Uplink,
		UEAMBRDownlink:  ambr.Downlink,
	}

	if netCap, ok := ue.EPSNetworkCapability(); ok {
		req.UENetworkCapability = &netCap
	}

	return req, &mapped, nil
}

func S1APHandoverFailureCause(cause ngap.Cause) s1ap.Cause {
	if cause.RadioNetwork != nil && *cause.RadioNetwork == ngap.CauseRadioNetworkInsufficientUECapabilities {
		return s1ap.CauseRadioNetworkInsufficientUECapabilities.Cause()
	}

	switch {
	case cause.RadioNetwork != nil:
		switch *cause.RadioNetwork {
		case ngap.CauseRadioNetworkNoRadioResourcesInTargetCell:
			return s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause()
		case ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported:
			return s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause()
		case ngap.CauseRadioNetworkHoTargetNotAllowed:
			return s1ap.CauseRadioNetworkHOTargetNotAllowed.Cause()
		}
	case cause.Misc != nil:
		if *cause.Misc == ngap.CauseMiscOMIntervention {
			return s1ap.CauseMiscOMIntervention.Cause()
		}
	}

	return s1ap.CauseRadioNetworkHOFailureInTarget.Cause()
}

func NGAPHandoverFailureCause(cause s1ap.Cause) ngap.Cause {
	switch {
	case cause.RadioNetwork != nil:
		switch *cause.RadioNetwork {
		case s1ap.CauseRadioNetworkInsufficientUECapabilities:
			return ngap.CauseRadioNetworkInsufficientUECapabilities.Cause()
		case s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell:
			return ngap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause()
		case s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported:
			return ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause()
		case s1ap.CauseRadioNetworkHOTargetNotAllowed:
			return ngap.CauseRadioNetworkHoTargetNotAllowed.Cause()
		}
	case cause.Misc != nil:
		if *cause.Misc == s1ap.CauseMiscOMIntervention {
			return ngap.CauseMiscOMIntervention.Cause()
		}
	}

	return ngap.CauseRadioNetworkHOFailureInTarget.Cause()
}

func NGAPHandoverCause(cause s1ap.Cause) ngap.Cause {
	if cause.RadioNetwork != nil {
		switch *cause.RadioNetwork {
		case s1ap.CauseRadioNetworkTimeCriticalHandover:
			return ngap.CauseRadioNetworkTimeCriticalHandover.Cause()
		case s1ap.CauseRadioNetworkResourceOptimisationHandover:
			return ngap.CauseRadioNetworkResourceOptimisationHandover.Cause()
		case s1ap.CauseRadioNetworkReduceLoadInServingCell:
			return ngap.CauseRadioNetworkReduceLoadInServingCell.Cause()
		}
	}

	return ngap.CauseRadioNetworkHandoverDesirableForRadio.Cause()
}

// TS 29.010 §7.2, TS 29.274 §8.49
func S1APHandoverCause(cause *ngap.Cause) s1ap.Cause {
	if cause != nil && cause.RadioNetwork != nil {
		switch *cause.RadioNetwork {
		case ngap.CauseRadioNetworkHandoverDesirableForRadio:
			return s1ap.CauseRadioNetworkHandoverDesirableForRadio.Cause()
		case ngap.CauseRadioNetworkTimeCriticalHandover:
			return s1ap.CauseRadioNetworkTimeCriticalHandover.Cause()
		case ngap.CauseRadioNetworkReduceLoadInServingCell:
			return s1ap.CauseRadioNetworkReduceLoadInServingCell.Cause()
		}
	}

	return s1ap.CauseRadioNetworkResourceOptimisationHandover.Cause()
}
