// Copyright 2024 Ella Networks
package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"

	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	errMandatoryIeMissing       = fmt.Errorf("mandatory IE missing")
	errNoEstablishedAssociation = fmt.Errorf("no established association")
)

var tracer = otel.Tracer("ella-core/upf")

type UpfPfcpHandler struct{}

func (u UpfPfcpHandler) HandlePfcpSessionEstablishmentRequest(ctx context.Context, msg *message.SessionEstablishmentRequest) (*message.SessionEstablishmentResponse, error) {
	return HandlePfcpSessionEstablishmentRequest(ctx, msg)
}

func (u UpfPfcpHandler) HandlePfcpSessionDeletionRequest(ctx context.Context, msg *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error) {
	return HandlePfcpSessionDeletionRequest(ctx, msg)
}

func (u UpfPfcpHandler) HandlePfcpSessionModificationRequest(ctx context.Context, msg *message.SessionModificationRequest) (*message.SessionModificationResponse, error) {
	return HandlePfcpSessionModificationRequest(ctx, msg)
}

func HandlePfcpSessionEstablishmentRequest(ctx context.Context, msg *message.SessionEstablishmentRequest) (*message.SessionEstablishmentResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/establish_session",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	conn := GetConnection()
	if conn == nil {
		err := fmt.Errorf("no connection")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no connection to UPF core")

		return nil, err
	}

	remoteSEID, err := validateRequest(msg.NodeID, msg.CPFSEID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "request validation failed")
		logger.WithTrace(ctx, logger.UpfLog).Info("Rejecting Session Establishment Request", zap.Error(err))

		return message.NewSessionEstablishmentResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), convertErrorToIeCause(err)), nil
	}

	seid := remoteSEID.SEID

	session := NewSession(seid)

	logger.WithTrace(ctx, logger.UpfLog).Debug("Tracking new session", logger.SEID(seid))

	var imsiStr string

	imsiUint64, err := extractImsiFromUserID(msg)
	if err != nil {
		logger.UpfLog.Warn("Could not extract IMSI from User ID IE", zap.Error(err), zap.Any("UserID", msg.UserID))
	} else {
		imsiStr = strconv.FormatUint(imsiUint64, 10)
	}

	printSessionEstablishmentRequest(msg)

	createdPDRs := []SPDRInfo{}
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	// Build in-memory FAR and QER maps for injection into PDRs below.
	farMap := make(map[uint32]ebpf.FarInfo)
	qerMap := make(map[uint32]ebpf.QerInfo)

	err = func() error {
		bpfObjects := conn.BpfObjects
		for _, far := range msg.CreateFAR {
			farInfo, err := composeFarInfo(far, conn.n3Address.To4(), ebpf.FarInfo{})
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}

			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

			session.NewFar(farid, farInfo)
			farMap[farid] = farInfo

			logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule", logger.FARID(farid), zap.Any("farInfo", farInfo))
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}

			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("qer id is missing")
			}

			updateQer(&qerInfo, qer)

			session.NewQer(qerID, qerInfo)
			qerMap[qerID] = qerInfo

			logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule", logger.QERID(qerID), zap.Any("qerInfo", qerInfo))
		}

		for _, urr := range msg.CreateURR {
			urrId, err := urr.URRID()
			if err != nil {
				return fmt.Errorf("URR ID missing")
			}

			if !urr.HasVOLUM() {
				return fmt.Errorf("only Volume Measurement Method is supported, received")
			}

			measurementPeriod, err := urr.MeasurementPeriod()
			if err != nil {
				return fmt.Errorf("measurement period is invalid: %s", err.Error())
			}

			err = bpfObjects.NewUrr(urrId)
			if err != nil {
				return fmt.Errorf("can't put URR: %s", err.Error())
			}

			logger.WithTrace(ctx, logger.UpfLog).Debug(
				"Received Usage Reporting Rule create",
				logger.URRID(urrId),
				zap.String("measurement_method", "Volume"),
				zap.Duration("measurement_period", measurementPeriod),
			)
		}

		for _, pdr := range msg.CreatePDR {
			// PDR should be created last, because we need to reference FARs and QERs global id
			pdrID, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := SPDRInfo{PdrID: uint32(pdrID), PdrInfo: ebpf.PdrInfo{SEID: seid, PdrID: uint32(pdrID), IMSI: imsiStr}}

			err = pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(spdrInfo.PdrID, spdrInfo)

			err = applyPDR(spdrInfo, bpfObjects)
			if err != nil {
				if spdrInfo.Allocated {
					pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
				}

				return fmt.Errorf("couldn't apply PDR: %s", err.Error())
			}

			logger.WithTrace(ctx, logger.UpfLog).Info("Applied packet detection rule", logger.PDRID(spdrInfo.PdrID))
			createdPDRs = append(createdPDRs, spdrInfo)
			bpfObjects.ClearNotified(seid, pdrID, spdrInfo.PdrInfo.Qer.Qfi)
		}

		return nil
	}()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to process PFCP IEs")
		logger.WithTrace(ctx, logger.UpfLog).Info("Rejecting Session Establishment Request (error in applying IEs)", zap.Error(err))

		return message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	conn.AddSession(seid, session)

	additionalIEs := []*ie.IE{
		newIeNodeID(conn.nodeID),
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewFSEID(seid, cloneIP(conn.nodeAddrV4), nil),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, cloneIP(conn.advertisedN3Address))
	additionalIEs = append(additionalIEs, pdrIEs...)

	estResp := message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, additionalIEs...)

	logger.WithTrace(ctx, logger.UpfLog).Debug("Accepted Session Establishment Request")

	return estResp, nil
}

func HandlePfcpSessionDeletionRequest(ctx context.Context, msg *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/delete_session",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	conn := GetConnection()
	if conn == nil {
		err := fmt.Errorf("no connection")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no connection to UPF core")

		return nil, err
	}

	printSessionDeleteRequest(msg)

	session := conn.GetSession(msg.SEID())
	if session == nil {
		logger.WithTrace(ctx, logger.UpfLog).Info("Rejecting Session Deletion Request (unknown SEID)")
		return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseSessionContextNotFound)), nil
	}

	bpfObjects := conn.BpfObjects

	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)
	for _, pdrInfo := range session.ListPDRs() {
		if err := pdrContext.deletePDR(pdrInfo, bpfObjects); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to delete PDR")

			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}

	conn.DeleteSession(msg.SEID())

	logger.WithTrace(ctx, logger.UpfLog).Info("Deleted session", logger.SEID(msg.SEID()))

	conn.ReleaseResources(msg.SEID())

	return message.NewSessionDeletionResponse(0, 0, session.SEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRequestAccepted)), nil
}

func HandlePfcpSessionModificationRequest(ctx context.Context, msg *message.SessionModificationRequest) (*message.SessionModificationResponse, error) {
	ctx, span := tracer.Start(ctx, "upf/modify_session",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	conn := GetConnection()
	if conn == nil {
		err := fmt.Errorf("no connection")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no connection to UPF core")

		return nil, err
	}

	session := conn.GetSession(msg.SEID())
	if session == nil {
		logger.WithTrace(ctx, logger.UpfLog).Info("Rejecting Session Modification Request (unknown SEID)")
		return message.NewSessionModificationResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseSessionContextNotFound)), nil
	}

	// This IE shall be present if the CP function decides to change its F-SEID for the PFCP session. The UP function
	// shall use the new CP F-SEID for subsequent PFCP Session related messages for this PFCP Session
	if msg.CPFSEID != nil {
		remoteSEID, err := msg.CPFSEID.FSEID()
		if err == nil {
			session.SEID = remoteSEID.SEID
			conn.AddSession(session.SEID, session)
		}
	}

	printSessionModificationRequest(msg)

	createdPDRs := []SPDRInfo{}
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	err := func() error {
		bpfObjects := conn.BpfObjects

		for _, far := range msg.CreateFAR {
			farInfo, err := composeFarInfo(far, conn.n3Address.To4(), ebpf.FarInfo{})
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}

			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

			session.NewFar(farid, farInfo)

			logger.WithTrace(ctx, logger.UpfLog).Info("Created Forwarding Action Rule", logger.FARID(farid), zap.Any("farInfo", farInfo))
		}

		for _, far := range msg.UpdateFAR {
			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			sFarInfo := session.GetFar(farid)

			sFarInfo, err = composeFarInfo(far, conn.n3Address.To4(), sFarInfo)
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}

			go addRemoteIPToNeigh(ctx, sFarInfo.RemoteIP)

			session.UpdateFar(farid, sFarInfo)

			// Re-apply all PDRs that reference this FAR with the updated embedded FAR.
			for _, spdrInfo := range session.ListPDRs() {
				if spdrInfo.PdrInfo.FarID == farid {
					spdrInfo.PdrInfo.Far = sFarInfo
					session.PutPDR(spdrInfo.PdrID, spdrInfo)

					if err := applyPDR(spdrInfo, bpfObjects); err != nil {
						return fmt.Errorf("can't update PDR after FAR update: %s", err.Error())
					}
				}
			}

			logger.WithTrace(ctx, logger.UpfLog).Info("Updated Forwarding Action Rule", logger.FARID(farid), zap.Any("farInfo", sFarInfo))
		}

		for _, far := range msg.RemoveFAR {
			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			session.RemoveFar(farid)

			logger.WithTrace(ctx, logger.UpfLog).Debug("Removed Forwarding Action Rule", logger.FARID(farid))
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}

			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing")
			}

			updateQer(&qerInfo, qer)

			session.NewQer(qerID, qerInfo)

			// Re-apply all PDRs that reference this QER with the updated embedded QER.
			for _, spdrInfo := range session.ListPDRs() {
				if spdrInfo.PdrInfo.QerID == qerID {
					spdrInfo.PdrInfo.Qer = qerInfo
					session.PutPDR(spdrInfo.PdrID, spdrInfo)

					if err := applyPDR(spdrInfo, bpfObjects); err != nil {
						return fmt.Errorf("can't apply PDR for new QER: %s", err.Error())
					}
				}
			}

			logger.WithTrace(ctx, logger.UpfLog).Info("Created QoS Enforcement Rule", logger.QERID(qerID), zap.Any("qerInfo", qerInfo))
		}

		for _, qer := range msg.UpdateQER {
			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}

			// Build updated QerInfo from the IE.
			sQerInfo := session.GetQer(qerID)
			updateQer(&sQerInfo, qer)

			session.NewQer(qerID, sQerInfo)

			// Re-apply all PDRs that reference this QER with the updated embedded QER.
			for _, spdrInfo := range session.ListPDRs() {
				if spdrInfo.PdrInfo.QerID == qerID {
					spdrInfo.PdrInfo.Qer = sQerInfo
					session.PutPDR(spdrInfo.PdrID, spdrInfo)

					if err := applyPDR(spdrInfo, bpfObjects); err != nil {
						return fmt.Errorf("can't update PDR after QER update: %s", err.Error())
					}
				}
			}
		}

		for _, qer := range msg.RemoveQER {
			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}

			logger.WithTrace(ctx, logger.UpfLog).Debug("Received QER remove (no-op for embedded QER)", logger.QERID(qerID))
		}

		for _, urr := range msg.CreateURR {
			urrId, err := urr.URRID()
			if err != nil {
				return fmt.Errorf("URR ID missing")
			}

			if !urr.HasVOLUM() {
				return fmt.Errorf("only Volume Measurement Method is supported, received")
			}

			measurementPeriod, err := urr.MeasurementPeriod()
			if err != nil {
				return fmt.Errorf("measurement period is invalid: %s", err.Error())
			}

			logger.WithTrace(ctx, logger.UpfLog).Debug(
				"Received Usage Reporting Rule create",
				logger.URRID(urrId),
				zap.String("measurement_method", "Volume"),
				zap.Duration("measurementPeriod", measurementPeriod),
			)
		}

		for _, urr := range msg.UpdateURR {
			urrId, err := urr.URRID()
			if err != nil {
				return fmt.Errorf("URR ID missing")
			}

			if !urr.HasVOLUM() {
				return fmt.Errorf("only Volume Measurement Method is supported, received")
			}

			measurementPeriod, err := urr.MeasurementPeriod()
			if err != nil {
				return fmt.Errorf("measurement period is invalid: %s", err.Error())
			}

			logger.WithTrace(ctx, logger.UpfLog).Debug(
				"Received Usage Reporting Rule update - Not yet supported",
				logger.URRID(urrId),
				zap.String("measurement_method", "Volume"),
				zap.Duration("measurementPeriod", measurementPeriod),
			)
		}

		for _, urr := range msg.RemoveURR {
			urrId, err := urr.URRID()
			if err != nil {
				return fmt.Errorf("URR ID missing")
			}

			logger.WithTrace(ctx, logger.UpfLog).Debug("Received Usage Reporting Rule remove - Not yet supported", logger.URRID(urrId))
		}

		// Build in-memory FAR and QER maps from the session for PDR injection.
		farMap := make(map[uint32]ebpf.FarInfo)
		for id, info := range session.ListFARs() {
			farMap[id] = info
		}

		qerMap := make(map[uint32]ebpf.QerInfo)
		for id, info := range session.ListQERs() {
			qerMap[id] = info
		}

		for _, pdr := range msg.CreatePDR {
			// PDR should be created last, because we need to reference FARs and QERs global id
			pdrID, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := SPDRInfo{PdrID: uint32(pdrID), PdrInfo: ebpf.PdrInfo{SEID: msg.SEID(), PdrID: uint32(pdrID)}}

			err = pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(spdrInfo.PdrID, spdrInfo)

			err = applyPDR(spdrInfo, bpfObjects)
			if err != nil {
				if spdrInfo.Allocated {
					pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
				}

				return fmt.Errorf("couldn't apply PDR: %s", err.Error())
			}

			createdPDRs = append(createdPDRs, spdrInfo)
			bpfObjects.ClearNotified(msg.SEID(), pdrID, spdrInfo.PdrInfo.Qer.Qfi)
		}

		for _, pdr := range msg.UpdatePDR {
			pdrID, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := session.GetPDR(pdrID)

			err = pdrContext.ExtractPDR(pdr, &spdrInfo, farMap, qerMap)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(uint32(pdrID), spdrInfo)

			err = applyPDR(spdrInfo, bpfObjects)
			if err != nil {
				if spdrInfo.Allocated {
					pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
				}

				return fmt.Errorf("couldn't apply PDR: %s", err.Error())
			}

			bpfObjects.ClearNotified(msg.SEID(), pdrID, spdrInfo.PdrInfo.Qer.Qfi)
		}

		for _, pdr := range msg.RemovePDR {
			pdrID, _ := pdr.PDRID()
			if session.HasPDR(uint32(pdrID)) {
				sPDRInfo := session.RemovePDR(uint32(pdrID))

				if err := pdrContext.deletePDR(sPDRInfo, bpfObjects); err != nil {
					return fmt.Errorf("couldn't delete PDR: %s", err.Error())
				}
			}
		}

		logger.WithTrace(ctx, logger.UpfLog).Debug("Session modification successful")

		return nil
	}()
	if err != nil {
		logger.WithTrace(ctx, logger.UpfLog).Info("Rejecting Session Modification Request (failed to apply rules)", zap.Error(err))
		return message.NewSessionModificationResponse(0, 0, session.SEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	conn.AddSession(session.SEID, session)

	additionalIEs := []*ie.IE{
		ie.NewCause(ie.CauseRequestAccepted),
		newIeNodeID(conn.nodeID),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, conn.advertisedN3Address)
	additionalIEs = append(additionalIEs, pdrIEs...)

	modResp := message.NewSessionModificationResponse(0, 0, session.SEID, msg.Sequence(), 0, additionalIEs...)

	return modResp, nil
}

func convertErrorToIeCause(err error) *ie.IE {
	switch err {
	case errMandatoryIeMissing:
		return ie.NewCause(ie.CauseMandatoryIEMissing)
	case errNoEstablishedAssociation:
		return ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)
	default:
		logger.UpfLog.Error("unknown PFCP error", zap.Error(err))
		return ie.NewCause(ie.CauseRequestRejected)
	}
}

func validateRequest(nodeID *ie.IE, cpfseid *ie.IE) (fseid *ie.FSEIDFields, err error) {
	if nodeID == nil || cpfseid == nil {
		return nil, errMandatoryIeMissing
	}

	_, err = nodeID.NodeID()
	if err != nil {
		return nil, errMandatoryIeMissing
	}

	fseid, err = cpfseid.FSEID()
	if err != nil {
		return nil, errMandatoryIeMissing
	}

	return fseid, nil
}

func IndexFunc[S ~[]E, E any](s S, f func(E) bool) int {
	for i := range s {
		if f(s[i]) {
			return i
		}
	}

	return -1
}

func findIEindex(ieArr []*ie.IE, ieType uint16) int {
	arrIndex := IndexFunc(ieArr, func(ie *ie.IE) bool {
		return ie.Type == ieType
	})

	return arrIndex
}

func causeToString(cause uint8) string {
	switch cause {
	case ie.CauseRequestAccepted:
		return "RequestAccepted"
	case ie.CauseRequestRejected:
		return "RequestRejected"
	case ie.CauseSessionContextNotFound:
		return "SessionContextNotFound"
	case ie.CauseMandatoryIEMissing:
		return "MandatoryIEMissing"
	case ie.CauseConditionalIEMissing:
		return "ConditionalIEMissing"
	case ie.CauseInvalidLength:
		return "InvalidLength"
	case ie.CauseMandatoryIEIncorrect:
		return "MandatoryIEIncorrect"
	case ie.CauseInvalidForwardingPolicy:
		return "InvalidForwardingPolicy"
	case ie.CauseInvalidFTEIDAllocationOption:
		return "InvalidFTEIDAllocationOption"
	case ie.CauseNoEstablishedPFCPAssociation:
		return "NoEstablishedPFCPAssociation"
	case ie.CauseRuleCreationModificationFailure:
		return "RuleCreationModificationFailure"
	case ie.CausePFCPEntityInCongestion:
		return "PFCPEntityInCongestion"
	case ie.CauseNoResourcesAvailable:
		return "NoResourcesAvailable"
	case ie.CauseServiceNotSupported:
		return "ServiceNotSupported"
	case ie.CauseSystemFailure:
		return "SystemFailure"
	case ie.CauseRedirectionRequested:
		return "RedirectionRequested"
	default:
		return "UnknownCause"
	}
}

func cloneIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)

	return dup
}

func composeFarInfo(far *ie.IE, localIP net.IP, farInfo ebpf.FarInfo) (ebpf.FarInfo, error) {
	if localIP == nil {
		return ebpf.FarInfo{}, fmt.Errorf("local IP is nil")
	}

	farInfo.LocalIP = binary.LittleEndian.Uint32(localIP)

	if applyAction, err := far.ApplyAction(); err == nil {
		farInfo.Action = applyAction[0]
	}

	var (
		forward []*ie.IE
		err     error
	)

	switch far.Type {
	case ie.CreateFAR:
		forward, err = far.ForwardingParameters()
	case ie.UpdateFAR:
		forward, err = far.UpdateForwardingParameters()
	default:
		return ebpf.FarInfo{}, fmt.Errorf("unsupported IE type")
	}

	if err == nil {
		outerHeaderCreationIndex := findIEindex(forward, 84) // IE Type Outer Header Creation
		if outerHeaderCreationIndex == -1 {
			logger.UpfLog.Debug("No outer header creation found")
		} else {
			outerHeaderCreation, _ := forward[outerHeaderCreationIndex].OuterHeaderCreation()
			farInfo.OuterHeaderCreation = uint8(outerHeaderCreation.OuterHeaderCreationDescription >> 8)

			farInfo.TeID = outerHeaderCreation.TEID
			if outerHeaderCreation.HasIPv4() {
				farInfo.RemoteIP = binary.LittleEndian.Uint32(outerHeaderCreation.IPv4Address)
			}

			if outerHeaderCreation.HasIPv6() {
				logger.UpfLog.Warn("IPv6 not supported yet, ignoring")
				return ebpf.FarInfo{}, fmt.Errorf("IPv6 not supported yet")
			}
		}
	}

	return farInfo, nil
}

func updateQer(qerInfo *ebpf.QerInfo, qer *ie.IE) {
	gateStatusDL, err := qer.GateStatusDL()
	if err == nil {
		qerInfo.GateStatusDL = gateStatusDL
	}

	gateStatusUL, err := qer.GateStatusUL()
	if err == nil {
		qerInfo.GateStatusUL = gateStatusUL
	}

	maxBitrateDL, err := qer.MBRDL()
	if err == nil {
		qerInfo.MaxBitrateDL = maxBitrateDL * 1000
	}

	maxBitrateUL, err := qer.MBRUL()
	if err == nil {
		qerInfo.MaxBitrateUL = maxBitrateUL * 1000
	}

	qfi, err := qer.QFI()
	if err == nil {
		qerInfo.Qfi = qfi
	}

	qerInfo.StartUL = 0
	qerInfo.StartDL = 0
}

func newIeNodeID(nodeID string) *ie.IE {
	ip := net.ParseIP(nodeID)
	if ip != nil {
		if ip.To4() != nil {
			return ie.NewNodeID(nodeID, "", "")
		}

		return ie.NewNodeID("", nodeID, "")
	}

	return ie.NewNodeID("", "", nodeID)
}

func extractImsiFromUserID(msg *message.SessionEstablishmentRequest) (uint64, error) {
	if msg == nil {
		return 0, fmt.Errorf("message is nil")
	}

	if msg.UserID == nil {
		return 0, fmt.Errorf("user ID IE is missing")
	}

	ieElem := msg.UserID

	userIDFields, err := ieElem.UserID()
	if err != nil {
		return 0, fmt.Errorf("failed to parse User ID IE: %w", err)
	}

	if userIDFields == nil || userIDFields.IMSI == "" {
		return 0, fmt.Errorf("IMSI is missing in User ID IE")
	}

	imsiUint64, err := strconv.ParseUint(userIDFields.IMSI, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to convert IMSI string to uint64: %w", err)
	}

	return imsiUint64, nil
}

// addRemoteIPToNeigh adds the given remote IP (encoded as a little-endian uint32)
// to the kernel neighbour table so that GTP encapsulated packets can be forwarded.
func addRemoteIPToNeigh(ctx context.Context, remoteIP uint32) {
	if remoteIP == 0 {
		return
	}

	ip_bytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(ip_bytes, remoteIP)
	ip := net.IP(ip_bytes)

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", logger.IPAddress(ip.String()), zap.Error(err))
	}
}
