// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// causeUnknownPLMN is NGAP Cause Misc "unknown-PLMN-or-SNPN" (TS 38.413),
// returned in NG Setup Failure when the gNB broadcasts no PLMN this AMF serves.
var causeUnknownPLMN = ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnknownPLMNOrSNPN}

// causeNoServedTAC is NGAP Cause Misc "unspecified", returned when the gNB
// broadcasts a served PLMN but no TAC this AMF serves.
var causeNoServedTAC = ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}

// causeUnspecified is the same Cause Misc "unspecified" on the wire, named for
// the other reason the AMF sends it: a failure on its own side, where the
// gNB's configuration is not at fault and nothing more specific fits. Kept
// distinct from causeNoServedTAC so a reader can tell which rejection a call
// site means.
var causeUnspecified = ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}

// HandleNGSetupRequest answers a gNB's NG Setup Request with an NG Setup
// Response when the gNB broadcasts a TAI this AMF serves, otherwise an NG Setup
// Failure (TS 38.413 §8.7.1).
func HandleNGSetupRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, req *ngap.NGSetupRequest) {
	name := ranNodeName(req.RANNodeName)

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))
		sendNGSetupFailure(ctx, ran, causeUnspecified, nil)

		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not list operator SNSSAI", zap.Error(err))
		sendNGSetupFailure(ctx, ran, causeUnspecified, nil)

		return
	}

	advertisedCapacity := amfInstance.RelativeCapacity()

	tais, outBytes, accepted, reason, err := ngSetupOutcomeFor(req, operatorInfo, snssaiList, amfInstance.Name, advertisedCapacity)
	if err != nil {
		// §8.7.1.3 obliges an answer whenever the AMF cannot accept the setup,
		// which includes being unable to build its own response.
		logger.WithTrace(ctx, ran.Log).Error("failed to handle NG Setup Request", zap.Error(err))
		sendNGSetupFailure(ctx, ran, causeUnspecified, nil)

		return
	}

	if !accepted {
		ran.SendToRadio(ctx, amf.NGAPProcedureNGSetupFailure, outBytes)

		logger.WithTrace(ctx, ran.Log).Warn("NG Setup rejected",
			zap.String("gnb-name", name),
			zap.String("reason", reason),
			zap.Any("gnb_tai_list", tais),
			zap.Any("core_tai_list", operatorInfo.Tais))

		return
	}

	// A gNB serving none of the operator's slices still completes NG Setup; it
	// just cannot carry any PDU session, so this is a warning rather than a
	// rejection (TS 38.413 §8.7.1 lists no cause for it).
	if !anySliceOverlap(tais, snssaiList) {
		logger.WithTrace(ctx, ran.Log).Warn("gNB advertises no S-NSSAIs overlapping with operator",
			zap.Any("gnb_tai_list", tais), zap.Any("core_slices", snssaiList))
	}

	if name != "" {
		amfInstance.UpdateRadioName(ran, name)
	}

	amfInstance.UpdateRadioSupportedTAIs(ran, tais)

	// Claim RanID only after validation passes; the dispatcher's
	// ran.RanID != nil guard gates all other NGAP handlers. The claim precedes
	// the response because evicting a duplicate association aborts it, and the
	// answer must not go out while the superseded one is still live.
	evicted := amfInstance.ClaimRanID(ran, req.GlobalRANNodeID, advertisedCapacity)
	if evicted != nil {
		logger.WithTrace(ctx, ran.Log).Warn("Evicted existing NG-C association with duplicate Global RAN Node ID",
			zap.String("evicted_remote", amf.AddrString(evicted.RemoteAddr())),
			zap.String("evicted_name", evicted.NodeName()),
		)
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureNGSetupResponse, outBytes)

	logger.WithTrace(ctx, ran.Log).Info("Radio completed NG Setup", zap.String("name", name))
}

// sendNGSetupFailure answers with an NG SETUP FAILURE carrying cause and, where
// the rejection answers a protocol error, the per-IE diagnostics §10.3.5 wants.
func sendNGSetupFailure(ctx context.Context, ran *amf.Radio, cause ngap.Cause, diag *ngap.CriticalityDiagnostics) {
	pkt, err := (&ngap.NGSetupFailure{Cause: &cause, CriticalityDiagnostics: diag}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building NG Setup Failure", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureNGSetupFailure, pkt)
}

// sendNGSetupProtocolFailure rejects a request that must not reach the
// application. NG Setup defines an unsuccessful outcome, so TS 38.413 §10.3.5
// answers with it rather than with the Error Indication other procedures use.
func sendNGSetupProtocolFailure(ctx context.Context, ran *amf.Radio, ase *ngap.AbstractSyntaxError) {
	diag := ase.OutcomeDiagnostics()

	sendNGSetupFailure(ctx, ran, ase.Cause, &diag)

	logger.WithTrace(ctx, ran.Log).Warn("NG Setup rejected", zap.Error(ase))
}

// ngSetupOutcomeFor returns an NG Setup Response when the gNB broadcasts a
// served TAI, otherwise an NG Setup Failure (TS 38.413 §8.7.1). reason is a
// human-readable rejection summary, empty when accepted; tais is what the gNB
// broadcasts, which the caller commits to the Radio only on accept.
func ngSetupOutcomeFor(req *ngap.NGSetupRequest, operatorInfo *amf.OperatorInfo, snssaiList []models.Snssai, amfName string, relativeCapacity uint8) (tais []amf.SupportedTAI, out []byte, accepted bool, reason string, err error) {
	tais = supportedTAIs(req.SupportedTAList)

	if cause, ok := servedTAICause(tais, operatorInfo); !ok {
		out, err = (&ngap.NGSetupFailure{Cause: &cause}).Marshal()
		if err != nil {
			return tais, nil, false, "", fmt.Errorf("amf: marshal NG Setup Failure: %w", err)
		}

		reason = "gNB broadcasts no PLMN served by this AMF (Unknown PLMN)"
		if cause == causeNoServedTAC {
			reason = "gNB broadcasts a served PLMN but no TAC served by this AMF"
		}

		if len(tais) == 0 {
			reason = "gNB broadcasts no supported TA"
		}

		return tais, out, false, reason, nil
	}

	resp, err := buildNGSetupResponse(operatorInfo.Guami, snssaiList, amfName, relativeCapacity)
	if err != nil {
		return tais, nil, false, "", err
	}

	// §10.3.4.2 reports in the response message of the procedure where it has
	// one. TS 38.413 assigns no notify criticality, so this stays empty today.
	if diag := req.Diagnostics(); diag.ReportRequired() {
		resp.CriticalityDiagnostics = &ngap.CriticalityDiagnostics{
			ProcedureCriticality:      ngap.Ptr(ngap.ProcedureCriticality(ngap.ProcNGSetup)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	out, err = resp.Marshal()
	if err != nil {
		return tais, nil, false, "", fmt.Errorf("amf: marshal NG Setup Response: %w", err)
	}

	return tais, out, true, "", nil
}

// servedTAICause reports whether the gNB broadcasts a TAI this AMF serves and,
// if not, the cause to reject with: "unknown-PLMN-or-SNPN" when no broadcast
// PLMN matches, otherwise "unspecified" when a served PLMN is broadcast but no
// served TAC. ok is true (cause unset) when a served TAI exists.
func servedTAICause(tais []amf.SupportedTAI, operatorInfo *amf.OperatorInfo) (cause ngap.Cause, ok bool) {
	if len(tais) == 0 {
		return causeNoServedTAC, false
	}

	if !amf.AnyPLMNMatch(tais, operatorInfo.Guami.PlmnID) {
		return causeUnknownPLMN, false
	}

	if !anyServedTAI(tais, operatorInfo.Tais) {
		return causeNoServedTAC, false
	}

	return ngap.Cause{}, true
}

// supportedTAIs flattens the gNB's Supported TA List into one entry per
// broadcast PLMN, which is how the AMF tracks what a radio serves. The TAC is
// rendered as six hex digits, matching the three-octet NR TAC.
func supportedTAIs(list ngap.SupportedTAList) []amf.SupportedTAI {
	tais := make([]amf.SupportedTAI, 0, len(list))

	for _, ta := range list {
		tac := fmt.Sprintf("%06x", uint32(ta.TAC))

		for _, bp := range ta.BroadcastPLMNList {
			plmnID := util.PLMNToModels(bp.PLMNIdentity)

			tai := amf.SupportedTAI{Tai: models.Tai{Tac: tac, PlmnID: &plmnID}}
			for _, s := range bp.TAISliceSupportList {
				tai.SNssaiList = append(tai.SNssaiList, util.SNSSAIToModels(s.SNSSAI))
			}

			tais = append(tais, tai)
		}
	}

	return tais
}

func anyServedTAI(tais []amf.SupportedTAI, served []models.Tai) bool {
	for _, tai := range tais {
		if amf.InTaiList(tai.Tai, served) {
			return true
		}
	}

	return false
}

func anySliceOverlap(tais []amf.SupportedTAI, operatorSlices []models.Snssai) bool {
	for _, tai := range tais {
		for _, gnbSlice := range tai.SNssaiList {
			for _, coreSlice := range operatorSlices {
				if gnbSlice.Equal(coreSlice) {
					return true
				}
			}
		}
	}

	return false
}

func buildNGSetupResponse(guami *models.Guami, snssaiList []models.Snssai, amfName string, relativeCapacity uint8) (*ngap.NGSetupResponse, error) {
	if guami == nil {
		return nil, fmt.Errorf("operator has no GUAMI")
	}

	g, err := util.GUAMIToNGAP(*guami)
	if err != nil {
		return nil, fmt.Errorf("error converting GUAMI to NGAP: %w", err)
	}

	slices := make(ngap.SliceSupportList, 0, len(snssaiList))

	for _, s := range snssaiList {
		snssai, err := util.SNSSAIToNGAP(s)
		if err != nil {
			return nil, fmt.Errorf("error converting S-NSSAI to NGAP: %w", err)
		}

		slices = append(slices, ngap.SliceSupportItem{SNSSAI: snssai})
	}

	return &ngap.NGSetupResponse{
		AMFName:             amfName,
		ServedGUAMIList:     ngap.ServedGUAMIList{{GUAMI: g}},
		RelativeAMFCapacity: ngap.Ptr(relativeCapacity),
		PLMNSupportList:     ngap.PLMNSupportList{{PLMNIdentity: g.PLMNIdentity, SliceSupportList: slices}},
	}, nil
}

func ranNodeName(name *string) string {
	if name == nil {
		return ""
	}

	return *name
}
