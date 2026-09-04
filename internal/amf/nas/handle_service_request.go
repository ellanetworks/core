// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type pendingN1 struct {
	pduSessionID uint8
	snssai       *models.Snssai
	n1Msg        []byte
	n2Info       []byte
}

type bufferedSM struct {
	stage        *pendingN1
	n1Only       []byte
	pduSessionID uint8
	stale        bool
	present      bool
	dropped      bool
}

func resolveBufferedSM(ue *amf.UeContext) bufferedSM {
	req := ue.N1N2Message()
	if req == nil || req.Standalone() {
		return bufferedSM{}
	}

	out := bufferedSM{pduSessionID: req.PduSessionID, present: true}

	if _, exist := ue.SmContextFindByPDUSessionID(req.PduSessionID); !exist {
		out.stale = true

		return out
	}

	if req.BinaryDataN2Information == nil {
		out.n1Only = req.BinaryDataN1Message

		return out
	}

	out.stage = &pendingN1{
		pduSessionID: req.PduSessionID,
		snssai:       req.SNssai,
		n1Msg:        req.BinaryDataN1Message,
		n2Info:       req.BinaryDataN2Information,
	}

	return out
}

func sendServiceAccept(
	ctx context.Context,
	guardCfg guard.TimerValue,
	ue *amf.UeContext,
	ueConn *amf.UeConn,
	initialContextSetup bool,
	proc amf.N2SetupProcedure,
	ctxList ngap.PDUSessionResourceSetupListCxtReq,
	suList ngap.PDUSessionResourceSetupListSUReq,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID []uint8,
	errCause []uint8,
	supportedGUAMI *models.Guami,
	pending *pendingN1,
) error {
	if initialContextSetup {
		if err := ue.UpdateSecurityContext(); err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	plain, err := amf.BuildServiceAccept(pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building service accept message: %v", err)
	}

	kgnb, ueSecCap := ue.Kgnb(), ue.UESecCap()

	var acceptWire []byte

	if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		acceptWire = wire

		return nil
	}); err != nil {
		amf.ReportProtectFailure(ctx, ue, "service accept", err)

		return err
	}

	var pendingWire []byte

	if pending != nil {
		pendingWire, err = stagePendingN1(ctx, ue, initialContextSetup, proc, pending, sht, &ctxList, &suList)
		if err != nil {
			amf.ReportProtectFailure(ctx, ue, "buffered N1 SM message", err)

			return err
		}
	}

	switch {
	case initialContextSetup:
		if err := ueConn.SendInitialContextSetup(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			kgnb,
			ue.RadioCapability,
			ue.RadioCapabilityForPaging,
			ueSecCap,
			acceptWire,
			ctxList,
			supportedGUAMI,
		); err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}

		ueConn.N2Setup(amf.N2SetupInitialContext).Arm(guardCfg)

		logger.From(ctx, logger.AmfLog).Info("sent service accept with initial context setup request")
	case len(suList) != 0:
		if err := ueConn.SendPDUSessionResourceSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			acceptWire,
			suList,
		); err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}

		ueConn.N2Setup(amf.N2SetupPDUSession).Arm(guardCfg)

		logger.From(ctx, logger.AmfLog).Info("sent service accept")
	default:
		ueConn.EndN2Setup(proc)

		if err := ueConn.SendDownlinkNASTransport(ctx, acceptWire); err != nil {
			return fmt.Errorf("error sending downlink nas transport: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("sent service accept")
	}

	if len(pendingWire) != 0 {
		if err := ueConn.SendDownlinkNASTransport(ctx, pendingWire); err != nil {
			return fmt.Errorf("error sending buffered N1 SM message: %v", err)
		}
	}

	return nil
}

func stagePendingN1(
	ctx context.Context,
	ue *amf.UeContext,
	initialContextSetup bool,
	proc amf.N2SetupProcedure,
	pending *pendingN1,
	sht uint8,
	ctxList *ngap.PDUSessionResourceSetupListCxtReq,
	suList *ngap.PDUSessionResourceSetupListSUReq,
) ([]byte, error) {
	var standalone []byte

	stage := func(nasPdu []byte) error {
		conn := ue.Conn()
		if conn == nil || !conn.N2Setup(proc).ClaimSession(pending.pduSessionID) {
			logger.From(ctx, logger.AmfLog).Debug("delivering buffered N1 without a duplicate PDU session setup",
				zap.Uint8("pdu_session_id", pending.pduSessionID))

			if len(nasPdu) == 0 || conn == nil {
				return nil
			}

			standalone = nasPdu

			return nil
		}

		if initialContextSetup {
			item, err := amf.PDUSessionSetupItem(pending.pduSessionID, pending.snssai, nasPdu, pending.n2Info)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pending.pduSessionID))

				return nil
			}

			*ctxList = append(*ctxList, item)

			return nil
		}

		item, err := amf.PDUSessionSetupItemSUReq(pending.pduSessionID, pending.snssai, nasPdu, pending.n2Info)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pending.pduSessionID))

			return nil
		}

		*suList = append(*suList, item)

		return nil
	}

	if pending.n1Msg == nil {
		return nil, stage(nil)
	}

	plain, err := amf.BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, pending.n1Msg, new(fgs.PDUSessionID(pending.pduSessionID)), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("error building DL NAS transport message: %v", err)
	}

	if err := ue.SendDownlinkNAS(plain, sht, stage); err != nil {
		return nil, err
	}

	return standalone, nil
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	msg, err := fgs.ParseServiceRequest(plain)
	if !decoded(ctx, "ServiceRequest", err) {
		logger.From(ctx, logger.AmfLog).Warn("failed to decode Service Request", zap.Error(err))
		rejectService(ctx, ueConn, fgs.GMMCauseInvalidMandatoryInformation)

		return nasreply.Handled()
	}

	state := ue.State()
	if state != amf.Deregistered && state != amf.Registered {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Service Request message", zap.String("state", string(state)))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	// TS 24.501: reject service request from deregistered UE
	if state == amf.Deregistered {
		rejectService(ctx, ueConn, fgs.GMMCauseUEIdentityCannotBeDerived)
		return nasreply.Handled()
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	ue.StopPaging()
	conn.StopNASGuard()

	// TS 24.501: an integrity-protected SERVICE REQUEST carrying a NAS
	// message container holds the real initial NAS message in that container;
	// decipher it and use it in place of the outer message.
	if msg.NASMessageContainer != nil && (ue.SecurityContextIsValid() && integrityVerified) {
		contents := append([]byte(nil), msg.NASMessageContainer...)

		err := ue.DecryptUplinkContents(contents)
		if err != nil {
			ue.ClearSecured()
		} else {
			inner, err := fgs.ParseServiceRequest(contents)
			if !decoded(ctx, "ServiceRequest", err) {
				logger.From(ctx, logger.AmfLog).Warn("failed to decode service request NAS message container", zap.Error(err))
				rejectService(ctx, ueConn, fgs.GMMCauseInvalidMandatoryInformation)

				return nasreply.Handled()
			}

			msg = inner
		}
		// TS 33.501: protected initial NAS message that failed the integrity check.
		conn.SetRetransmissionOfInitialNASMsg(!integrityVerified)
	}

	// Service Reject if the SecurityContext is invalid. TS 24.501: a
	// service request failing the integrity check is rejected with 5GMM cause
	// #9 and the 5GMM-context and 5G NAS security context are left unchanged, so
	// an unauthenticated message cannot tear down a genuine UE's security state.
	if !ue.SecurityContextIsValid() || !integrityVerified {
		logger.From(ctx, logger.AmfLog).Warn("No valid security context for service request", logger.SUPI(ue.Supi().String()))

		rejectService(ctx, ueConn, fgs.GMMCauseUEIdentityCannotBeDerived)

		return nasreply.Handled()
	}

	serviceType := msg.ServiceType

	logger.WithTrace(ctx, logger.AmfLog).Debug("Handle Service Request", logger.SUPI(ue.Supi().String()), zap.String("service_type", serviceType.String()))

	var (
		reactivationResult, acceptPduSessionPsi *[16]bool
		errPduSessionID, errCause               []uint8
		targetPduSessionID                      uint8
	)

	var (
		suList  ngap.PDUSessionResourceSetupListSUReq
		ctxList ngap.PDUSessionResourceSetupListCxtReq
	)

	switch serviceType {
	case fgs.ServiceTypeEmergencyServices, fgs.ServiceTypeEmergencyServicesFallback:
		// Ella does not provide emergency services; the request cannot be accepted, so
		// answer SERVICE REJECT #7 "5GS services not allowed" rather than silently dropping
		// it (TS 24.501 §5.6.1.5).
		logger.From(ctx, logger.AmfLog).Warn("emergency service is not supported; rejecting service request")
		rejectService(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)

		return nasreply.Handled()
	case fgs.ServiceTypeSignalling, fgs.ServiceTypeElevatedSignalling,
		fgs.ServiceTypeData, fgs.ServiceTypeHighPriorityAccess,
		fgs.ServiceTypeMobileTerminatedServices:
	default:
		// TS 24.501 §5.6.1.5: a service request with an unsupported or unknown service type
		// cannot be accepted; answer SERVICE REJECT rather than silently dropping it.
		logger.From(ctx, logger.AmfLog).Warn("service type is not supported; rejecting", zap.Stringer("service_type", serviceType))
		rejectService(ctx, ueConn, fgs.GMMCauseProtocolErrorUnspecified)

		return nasreply.Handled()
	}

	operator, err := amfInstance.Operator(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error getting operator info", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonUnspecified)
	}

	operatorInfo := operator.Info()

	buffered := resolveBufferedSM(ue)

	if buffered.stale {
		logger.From(ctx, logger.AmfLog).Warn("discarding buffered downlink payload naming a PDU session the UE no longer holds",
			zap.Uint8("pdu_session_id", buffered.pduSessionID))
	}

	if buffered.stage != nil {
		targetPduSessionID = buffered.stage.pduSessionID
	}

	// Copy SmContextList under lock for safe concurrent iteration.
	smContextSnapshot := ue.SmContextSnapshot()

	if msg.PDUSessionStatus != nil {
		acceptPduSessionPsi = new([16]bool)

		psiArray := msg.PDUSessionStatus.PSI
		for pduSessionID, smContext := range smContextSnapshot {
			if int(pduSessionID) >= len(psiArray) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in PDUSessionStatus processing", zap.Uint8("pdu_session_id", pduSessionID))
				continue
			}

			if !psiArray[pduSessionID] {
				if err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref); err != nil {
					logger.From(ctx, logger.AmfLog).Error("Release amf.SmContext Error", zap.Error(err))
				}

				ue.DeleteSmContext(pduSessionID)
				delete(smContextSnapshot, pduSessionID)
			} else {
				acceptPduSessionPsi[pduSessionID] = true
			}
		}
	}

	if buffered.present && !buffered.stale {
		if _, held := smContextSnapshot[buffered.pduSessionID]; !held {
			logger.From(ctx, logger.AmfLog).Warn("discarding buffered downlink payload naming a PDU session the UE reports inactive",
				zap.Uint8("pdu_session_id", buffered.pduSessionID))

			buffered.stage = nil
			buffered.n1Only = nil
			buffered.dropped = true
		}
	}

	activate := make(map[uint8]bool, len(smContextSnapshot))

	if buffered.present && !buffered.stale && !buffered.dropped && buffered.stage == nil {
		activate[buffered.pduSessionID] = true
	}

	var (
		requestedPsi    []uint8
		alreadyOnTheRAN [16]bool
	)

	if msg.UplinkDataStatus != nil {
		uplinkDataPsi := msg.UplinkDataStatus.PSI
		reactivationResult = new([16]bool)

		for pduSessionID := range smContextSnapshot {
			if int(pduSessionID) >= len(uplinkDataPsi) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in UplinkDataStatus processing", zap.Uint8("pdu_session_id", pduSessionID))
				continue
			}

			if !uplinkDataPsi[pduSessionID] {
				continue
			}

			requestedPsi = append(requestedPsi, pduSessionID)

			if pduSessionID == targetPduSessionID {
				continue
			}

			activate[pduSessionID] = true
		}
	}

	proc, initialContextSetup := ueConn.ClaimN2Setup(buffered.stage != nil || len(activate) != 0)

	for pduSessionID := range activate {
		smContext, ok := smContextSnapshot[pduSessionID]
		if !ok {
			continue
		}

		failed := func(err error) {
			logger.From(ctx, logger.AmfLog).Error("could not re-establish user-plane resources", zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID))

			if reactivationResult != nil {
				reactivationResult[pduSessionID] = true
			}

			errPduSessionID = append(errPduSessionID, pduSessionID)
			errCause = append(errCause, uint8(fgs.GMMCauseProtocolErrorUnspecified))
		}

		if !ueConn.N2Setup(proc).ClaimSession(pduSessionID) {
			logger.From(ctx, logger.AmfLog).Debug("skipping PDU session already set up on the NG-RAN node",
				zap.Uint8("pdu_session_id", pduSessionID))

			if int(pduSessionID) < len(alreadyOnTheRAN) {
				alreadyOnTheRAN[pduSessionID] = true
			}

			continue
		}

		binaryDataN2SmInformation, err := amfInstance.Session.ActivateSmContext(ctx, smContext.Ref)
		if err != nil {
			failed(err)

			continue
		}

		if initialContextSetup {
			item, err := amf.PDUSessionSetupItem(pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
			if err != nil {
				failed(err)

				continue
			}

			ctxList = append(ctxList, item)

			continue
		}

		item, err := amf.PDUSessionSetupItemSUReq(pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
		if err != nil {
			failed(err)

			continue
		}

		suList = append(suList, item)
	}

	if buffered.stage == nil && len(ctxList) == 0 && len(suList) == 0 {
		var unestablished []uint8

		for _, pduSessionID := range requestedPsi {
			if int(pduSessionID) < len(alreadyOnTheRAN) && alreadyOnTheRAN[pduSessionID] {
				continue
			}

			unestablished = append(unestablished, pduSessionID)
			reactivationResult[pduSessionID] = true
		}

		if len(unestablished) != 0 {
			logger.From(ctx, logger.AmfLog).Error("no user-plane resources established for a service request that asked for them",
				logger.SUPI(ue.Supi().String()), zap.Uint8s("pdu_session_ids", unestablished))
		}
	}

	accept := func(pending *pendingN1) error {
		err := sendServiceAccept(ctx, amfInstance.N2SetupGuardCfg, ue, ueConn, initialContextSetup, proc, ctxList, suList, acceptPduSessionPsi,
			reactivationResult, errPduSessionID, errCause, operatorInfo.Guami, pending)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))

			if initialContextSetup {
				ueConn.AbortICS()
			} else {
				ueConn.EndN2Setup(amf.N2SetupPDUSession)
			}
		}

		return err
	}

	if buffered.stale && serviceType == fgs.ServiceTypeMobileTerminatedServices {
		ue.ClearN1N2Message()

		if initialContextSetup {
			ueConn.AbortICS()
		}

		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	if err := accept(buffered.stage); err != nil {
		return nasreply.Handled()
	}

	if buffered.present {
		ue.ClearN1N2Message()
	}

	if buffered.n1Only != nil {
		amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, buffered.n1Only, fgs.PDUSessionID(buffered.pduSessionID), 0)
	}

	if serviceType == fgs.ServiceTypeMobileTerminatedServices {
		// TS 24.501 requires assigning a new GUTI after a successful Service Request
		// triggered by a paging request.
		if err := amfInstance.ReallocateGUTI(ctx, ue); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error reallocating GUTI to UE", zap.Error(err))
			return nasreply.Handled()
		}

		amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, true, operator)
	}

	if len(errPduSessionID) != 0 {
		logger.From(ctx, logger.AmfLog).Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}

	return nasreply.Handled()
}

// rejectService answers a service request the AMF cannot accept with a SERVICE REJECT
// carrying cause, releasing the RAN connection only when TS 24.501 §5.3.1.3 expects it.
func rejectService(ctx context.Context, ueConn *amf.UeConn, cause fgs.GMMCause) {
	amf.SendServiceReject(ctx, ueConn, cause)

	if !releasesN1Connection(ueConn, cause) {
		return
	}

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease
	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
}

func releasesN1Connection(ueConn *amf.UeConn, cause fgs.GMMCause) bool {
	switch cause {
	case fgs.GMMCauseServicesNotAllowed,
		fgs.GMMCausePLMNNotAllowed,
		fgs.GMMCauseTrackingAreaNotAllowed,
		fgs.GMMCauseRoamingNotAllowedInThisTA,
		fgs.GMMCauseNoSuitableCellsInTrackingArea,
		fgs.GMMCauseN1ModeNotAllowed,
		fgs.GMMCauseRedirectionToEPCRequired,
		fgs.GMMCauseNoNetworkSlicesAvailable,
		fgs.GMMCauseNon3GPPAccessNotAllowed,
		fgs.GMMCauseServingNetworkNotAuthorized,
		fgs.GMMCauseTemporarilyNotAuthorizedForThisSNPN,
		fgs.GMMCausePermanentlyNotAuthorizedForThisSNPN,
		fgs.GMMCauseNotAuthorizedForThisCAG,
		fgs.GMMCausePLMNNotAllowedToOperateAtPresentUELocation,
		fgs.GMMCauseUEIdentityCannotBeDerived,
		fgs.GMMCauseImplicitlyDeregistered,
		fgs.GMMCauseRestrictedServiceArea:
		return true
	}

	return ueConn.SentFrom5GMMIdle()
}
