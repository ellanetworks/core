// Copyright 2024 Ella Networks
package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"go.opentelemetry.io/otel"
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
	ctx, span := tracer.Start(ctx, "UPF Session Establishment")
	defer span.End()

	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	remoteSEID, err := validateRequest(msg.NodeID, msg.CPFSEID)
	if err != nil {
		logger.UpfLog.Info("Rejecting Session Establishment Request", zap.Error(err))
		return message.NewSessionEstablishmentResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), convertErrorToIeCause(err)), nil
	}

	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Info("Rejecting Session Establishment Request (no association)", zap.String("smfAddress", conn.SmfAddress))
		return message.NewSessionEstablishmentResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}

	localSEID := association.NewLocalSEID()

	session := NewSession(localSEID, remoteSEID.SEID)
	logger.UpfLog.Debug("Tracking new session", zap.Uint64("Local SEID", localSEID), zap.Uint64("Remote SEID", remoteSEID.SEID))

	printSessionEstablishmentRequest(msg)
	createdPDRs := []SPDRInfo{}
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	err = func() error {
		bpfObjects := conn.bpfObjects
		for _, far := range msg.CreateFAR {
			farInfo, err := composeFarInfo(far, conn.n3Address.To4(), ebpf.FarInfo{})
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}

			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			err = bpfObjects.NewFar(ctx, farid, farInfo)
			if err != nil {
				return fmt.Errorf("can't put FAR: %s", err.Error())
			}

			session.NewFar(farid, farInfo)

			logger.UpfLog.Info("Created Forwarding Action Rule", zap.Uint32("farID", farid), zap.Any("farInfo", farInfo))
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}

			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("qer id is missing")
			}

			updateQer(&qerInfo, qer)

			err = bpfObjects.NewQer(qerID, qerInfo)
			if err != nil {
				return fmt.Errorf("can't put QER: %s", err.Error())
			}

			session.NewQer(qerID, qerInfo)

			logger.UpfLog.Info("Created QoS Enforcement Rule", zap.Uint32("qerID", qerID), zap.Any("qerInfo", qerInfo))
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

			logger.UpfLog.Debug(
				"Received Usage Reporting Rule create",
				zap.Uint32("urr_id", urrId),
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

			spdrInfo := SPDRInfo{PdrID: uint32(pdrID), PdrInfo: ebpf.PdrInfo{LocalSEID: localSEID, PdrID: uint32(pdrID)}}

			err = pdrContext.ExtractPDR(pdr, &spdrInfo)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(spdrInfo.PdrID, spdrInfo)
			applyPDR(spdrInfo, bpfObjects)
			logger.UpfLog.Info("Applied packet detection rule", zap.Uint32("pdrID", spdrInfo.PdrID))
			createdPDRs = append(createdPDRs, spdrInfo)
			bpfObjects.ClearNotified(localSEID, pdrID, session.GetQer(spdrInfo.PdrInfo.QerID).Qfi)
		}
		return nil
	}()
	if err != nil {
		logger.UpfLog.Info("Rejecting Session Establishment Request (error in applying IEs)", zap.Error(err))
		return message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	// Reassigning is the best I can think of for now
	association.Sessions[localSEID] = session
	conn.SmfNodeAssociation = association
	additionalIEs := []*ie.IE{
		newIeNodeID(conn.nodeID),
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewFSEID(localSEID, cloneIP(conn.nodeAddrV4), nil),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, cloneIP(conn.advertisedN3Address))
	additionalIEs = append(additionalIEs, pdrIEs...)

	estResp := message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, additionalIEs...)
	logger.UpfLog.Debug("Accepted Session Establishment Request", zap.String("smfAddress", conn.SmfAddress))
	return estResp, nil
}

func HandlePfcpSessionDeletionRequest(ctx context.Context, msg *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error) {
	_, span := tracer.Start(ctx, "UPF Session Delete")
	defer span.End()
	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Info("Rejecting Session Deletion Request (no association)", zap.String("smfAddress", conn.SmfAddress))
		return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}
	printSessionDeleteRequest(msg)

	session, ok := association.Sessions[msg.SEID()]
	if !ok {
		logger.UpfLog.Info("Rejecting Session Deletion Request (unknown SEID)", zap.String("smfAddress", conn.SmfAddress))
		return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseSessionContextNotFound)), nil
	}
	bpfObjects := conn.bpfObjects
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)
	for _, pdrInfo := range session.PDRs {
		if err := pdrContext.deletePDR(pdrInfo, bpfObjects); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	for id := range session.FARs {
		if err := bpfObjects.DeleteFar(id); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	for id := range session.QERs {
		if err := bpfObjects.DeleteQer(id); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	logger.UpfLog.Info("Deleting session", zap.Uint64("seid", msg.SEID()))
	delete(association.Sessions, msg.SEID())

	conn.ReleaseResources(msg.SEID())

	return message.NewSessionDeletionResponse(0, 0, session.RemoteSEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRequestAccepted)), nil
}

func HandlePfcpSessionModificationRequest(ctx context.Context, msg *message.SessionModificationRequest) (*message.SessionModificationResponse, error) {
	ctx, span := tracer.Start(ctx, "UPF Session Modify")
	defer span.End()

	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}

	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Info("Rejecting Session Modification Request (no association)", zap.String("smfAddress", conn.SmfAddress))
		return message.NewSessionModificationResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}

	session, ok := association.Sessions[msg.SEID()]
	if !ok {
		logger.UpfLog.Info("Rejecting Session Modification Request (unknown SEID)", zap.String("smfAddress", conn.SmfAddress))
		return message.NewSessionModificationResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseSessionContextNotFound)), nil
	}

	// This IE shall be present if the CP function decides to change its F-SEID for the PFCP session. The UP function
	// shall use the new CP F-SEID for subsequent PFCP Session related messages for this PFCP Session
	if msg.CPFSEID != nil {
		remoteSEID, err := msg.CPFSEID.FSEID()
		if err == nil {
			session.RemoteSEID = remoteSEID.SEID

			association.Sessions[msg.SEID()] = session
			conn.SmfNodeAssociation = association
		}
	}

	printSessionModificationRequest(msg)

	createdPDRs := []SPDRInfo{}
	pdrContext := NewPDRCreationContext(session, conn.FteIDResourceManager)

	err := func() error {
		bpfObjects := conn.bpfObjects

		for _, far := range msg.CreateFAR {
			farInfo, err := composeFarInfo(far, conn.n3Address.To4(), ebpf.FarInfo{})
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}

			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			err = bpfObjects.NewFar(ctx, farid, farInfo)
			if err != nil {
				return fmt.Errorf("can't put FAR: %s", err.Error())
			}

			session.NewFar(farid, farInfo)

			logger.UpfLog.Info("Created Forwarding Action Rule", zap.Uint32("farID", farid), zap.Any("farInfo", farInfo))
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

			session.UpdateFar(farid, sFarInfo)

			if err := bpfObjects.UpdateFar(ctx, farid, sFarInfo); err != nil {
				return fmt.Errorf("can't update FAR: %s", err.Error())
			}
		}

		for _, far := range msg.RemoveFAR {
			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}

			session.RemoveFar(farid)

			if err := bpfObjects.DeleteFar(farid); err != nil {
				return fmt.Errorf("can't remove FAR: %s", err.Error())
			}
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}

			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing")
			}

			updateQer(&qerInfo, qer)

			err = bpfObjects.NewQer(qerID, qerInfo)
			if err != nil {
				return fmt.Errorf("can't put QER: %s", err.Error())
			}

			session.NewQer(qerID, qerInfo)

			logger.UpfLog.Info("Created QoS Enforcement Rule", zap.Uint32("qerID", qerID), zap.Any("qerInfo", qerInfo))
		}

		for _, qer := range msg.UpdateQER {
			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}

			sQerInfo := session.GetQer(qerID)

			updateQer(&sQerInfo, qer)

			session.UpdateQer(qerID, sQerInfo)

			if err := bpfObjects.UpdateQer(qerID, sQerInfo); err != nil {
				return fmt.Errorf("can't update QER: %s", err.Error())
			}
		}

		for _, qer := range msg.RemoveQER {
			qerID, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}

			session.RemoveQer(qerID)

			if err := bpfObjects.DeleteQer(qerID); err != nil {
				return fmt.Errorf("can't remove QER: %s", err.Error())
			}
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

			logger.UpfLog.Debug(
				"Received Usage Reporting Rule create",
				zap.Uint32("urrID", urrId),
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
			logger.UpfLog.Debug(
				"Received Usage Reporting Rule update - Not yet supported",
				zap.Uint32("urrID", urrId),
				zap.String("measurement_method", "Volume"),
				zap.Duration("measurementPeriod", measurementPeriod),
			)
		}

		for _, urr := range msg.RemoveURR {
			urrId, err := urr.URRID()
			if err != nil {
				return fmt.Errorf("URR ID missing")
			}
			logger.UpfLog.Debug("Received Usage Reporting Rule remove - Not yet supported", zap.Uint32("urrID", urrId))
		}

		for _, pdr := range msg.CreatePDR {
			// PDR should be created last, because we need to reference FARs and QERs global id
			pdrID, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := SPDRInfo{PdrID: uint32(pdrID), PdrInfo: ebpf.PdrInfo{LocalSEID: msg.SEID(), PdrID: uint32(pdrID)}}

			err = pdrContext.ExtractPDR(pdr, &spdrInfo)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(spdrInfo.PdrID, spdrInfo)
			applyPDR(spdrInfo, bpfObjects)
			createdPDRs = append(createdPDRs, spdrInfo)
			bpfObjects.ClearNotified(msg.SEID(), pdrID, session.GetQer(spdrInfo.PdrInfo.QerID).Qfi)
		}

		for _, pdr := range msg.UpdatePDR {
			pdrID, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := session.GetPDR(pdrID)

			err = pdrContext.ExtractPDR(pdr, &spdrInfo)
			if err != nil {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}

			session.PutPDR(uint32(pdrID), spdrInfo)
			applyPDR(spdrInfo, bpfObjects)
			bpfObjects.ClearNotified(msg.SEID(), pdrID, session.GetQer(spdrInfo.PdrInfo.QerID).Qfi)
		}

		for _, pdr := range msg.RemovePDR {
			pdrID, _ := pdr.PDRID()
			if _, ok := session.PDRs[uint32(pdrID)]; ok {
				sPDRInfo := session.RemovePDR(uint32(pdrID))

				if err := pdrContext.deletePDR(sPDRInfo, bpfObjects); err != nil {
					return fmt.Errorf("couldn't delete PDR: %s", err.Error())
				}
			}
		}
		logger.UpfLog.Debug("Session modification successful")
		return nil
	}()
	if err != nil {
		logger.UpfLog.Info("Rejecting Session Modification Request (failed to apply rules)", zap.Error(err))
		return message.NewSessionModificationResponse(0, 0, session.RemoteSEID, msg.Sequence(), 0, newIeNodeID(conn.nodeID), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	association.Sessions[msg.SEID()] = session

	additionalIEs := []*ie.IE{
		ie.NewCause(ie.CauseRequestAccepted),
		newIeNodeID(conn.nodeID),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, conn.advertisedN3Address)
	additionalIEs = append(additionalIEs, pdrIEs...)

	modResp := message.NewSessionModificationResponse(0, 0, session.RemoteSEID, msg.Sequence(), 0, additionalIEs...)
	return modResp, nil
}

func convertErrorToIeCause(err error) *ie.IE {
	switch err {
	case errMandatoryIeMissing:
		return ie.NewCause(ie.CauseMandatoryIEMissing)
	case errNoEstablishedAssociation:
		return ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)
	default:
		logger.UpfLog.Info("Unknown error", zap.Error(err))
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

	var forward []*ie.IE
	var err error
	if far.Type == ie.CreateFAR {
		forward, err = far.ForwardingParameters()
	} else if far.Type == ie.UpdateFAR {
		forward, err = far.UpdateForwardingParameters()
	} else {
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
	transportLevelMarking, err := GetTransportLevelMarking(far)
	if err == nil {
		farInfo.TransportLevelMarking = transportLevelMarking
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

func GetTransportLevelMarking(far *ie.IE) (uint16, error) {
	for _, informationalElement := range far.ChildIEs {
		if informationalElement.Type == ie.TransportLevelMarking {
			return informationalElement.TransportLevelMarking()
		}
	}
	return 0, fmt.Errorf("no TransportLevelMarking found")
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
