// Copyright 2024 Ella Networks
package core

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/config"
	"github.com/ellanetworks/core/internal/upf/ebpf"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

var (
	errMandatoryIeMissing       = fmt.Errorf("mandatory IE missing")
	errNoEstablishedAssociation = fmt.Errorf("no established association")
)

func HandlePfcpSessionEstablishmentRequest(msg *message.SessionEstablishmentRequest) (*message.SessionEstablishmentResponse, error) {
	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	remoteSEID, err := validateRequest(msg.NodeID, msg.CPFSEID)
	if err != nil {
		logger.UpfLog.Infof("Rejecting Session Establishment Request from: %s (error in IEs)", err)
		return message.NewSessionEstablishmentResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), convertErrorToIeCause(err)), nil
	}

	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Infof("Rejecting Session Establishment Request from: %s (no association)", config.Conf.SmfAddress)
		return message.NewSessionEstablishmentResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}

	localSEID := association.NewLocalSEID()

	session := NewSession(localSEID, remoteSEID.SEID)

	printSessionEstablishmentRequest(msg)
	createdPDRs := []SPDRInfo{}
	pdrContext := NewPDRCreationContext(session, conn.ResourceManager)

	err = func() error {
		bpfObjects := conn.bpfObjects
		for _, far := range msg.CreateFAR {
			farInfo, err := composeFarInfo(far, conn.n3Address.To4(), ebpf.FarInfo{})
			if err != nil {
				logger.UpfLog.Infof("Error extracting FAR info: %s", err.Error())
				continue
			}

			farid, _ := far.FARID()
			logger.UpfLog.Infof("Saving FAR info to session: %d, %+v", farid, farInfo)
			if internalId, err := bpfObjects.NewFar(farInfo); err == nil {
				session.NewFar(farid, internalId, farInfo)
			} else {
				logger.UpfLog.Infof("Can't put FAR: %s", err.Error())
				return err
			}
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}
			qerId, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing")
			}
			updateQer(&qerInfo, qer)
			logger.UpfLog.Infof("Saving QER info to session: %d, %+v", qerId, qerInfo)
			if internalId, err := bpfObjects.NewQer(qerInfo); err == nil {
				session.NewQer(qerId, internalId, qerInfo)
			} else {
				logger.UpfLog.Infof("Can't put QER: %s", err.Error())
				return err
			}
		}

		for _, pdr := range msg.CreatePDR {
			// PDR should be created last, because we need to reference FARs and QERs global id
			pdrId, err := pdr.PDRID()
			if err != nil {
				continue
			}

			spdrInfo := SPDRInfo{PdrID: uint32(pdrId)}

			if err := pdrContext.ExtractPDR(pdr, &spdrInfo); err == nil {
				session.PutPDR(spdrInfo.PdrID, spdrInfo)
				applyPDR(spdrInfo, bpfObjects)
				createdPDRs = append(createdPDRs, spdrInfo)
			} else {
				logger.UpfLog.Errorf("couldn't extract PDR info: %s", err.Error())
			}
		}
		return nil
	}()
	if err != nil {
		logger.UpfLog.Infof("Rejecting Session Establishment Request from: %s (error in applying IEs)", err)
		return message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	// Reassigning is the best I can think of for now
	association.Sessions[localSEID] = session
	conn.SmfNodeAssociation = association
	additionalIEs := []*ie.IE{
		newIeNodeID(conn.nodeId),
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewFSEID(localSEID, cloneIP(conn.nodeAddrV4), nil),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, cloneIP(conn.n3Address))
	additionalIEs = append(additionalIEs, pdrIEs...)

	// Send SessionEstablishmentResponse
	estResp := message.NewSessionEstablishmentResponse(0, 0, remoteSEID.SEID, msg.Sequence(), 0, additionalIEs...)
	logger.UpfLog.Infof("Accepted Session Establishment Request from: %s", config.Conf.SmfAddress)
	return estResp, nil
}

func HandlePfcpSessionDeletionRequest(msg *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error) {
	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Infof("Rejecting Session Deletion Request from: %s (no association)", config.Conf.SmfAddress)
		return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}
	printSessionDeleteRequest(msg)

	session, ok := association.Sessions[msg.SEID()]
	if !ok {
		logger.UpfLog.Infof("Rejecting Session Deletion Request from: %s (unknown SEID)", config.Conf.SmfAddress)
		return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseSessionContextNotFound)), nil
	}
	bpfObjects := conn.bpfObjects
	pdrContext := NewPDRCreationContext(session, conn.ResourceManager)
	for _, pdrInfo := range session.PDRs {
		if err := pdrContext.deletePDR(pdrInfo, bpfObjects); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	for id := range session.FARs {
		if err := bpfObjects.DeleteFar(id); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	for id := range session.QERs {
		if err := bpfObjects.DeleteQer(id); err != nil {
			return message.NewSessionDeletionResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRuleCreationModificationFailure)), err
		}
	}
	logger.UpfLog.Infof("Deleting session: %d", msg.SEID())
	delete(association.Sessions, msg.SEID())

	conn.ReleaseResources(msg.SEID())

	return message.NewSessionDeletionResponse(0, 0, session.RemoteSEID, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRequestAccepted)), nil
}

func HandlePfcpSessionModificationRequest(msg *message.SessionModificationRequest) (*message.SessionModificationResponse, error) {
	conn := GetConnection()
	if conn == nil {
		return nil, fmt.Errorf("no connection")
	}

	association := conn.SmfNodeAssociation
	if association == nil {
		logger.UpfLog.Infof("Rejecting Session Modification Request from: %s (no association)", config.Conf.SmfAddress)
		return message.NewSessionModificationResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseNoEstablishedPFCPAssociation)), nil
	}

	session, ok := association.Sessions[msg.SEID()]
	if !ok {
		logger.UpfLog.Infof("Rejecting Session Modification Request from: %s (unknown SEID)", config.Conf.SmfAddress)
		return message.NewSessionModificationResponse(0, 0, 0, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseSessionContextNotFound)), nil
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
	pdrContext := NewPDRCreationContext(session, conn.ResourceManager)

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
			if internalId, err := bpfObjects.NewFar(farInfo); err == nil {
				session.NewFar(farid, internalId, farInfo)
			} else {
				return fmt.Errorf("can't put FAR: %s", err.Error())
			}
		}

		for _, far := range msg.UpdateFAR {
			farid, err := far.FARID()
			if err != nil {
				return fmt.Errorf("FAR ID missing: %s", err.Error())
			}
			sFarInfo := session.GetFar(farid)
			sFarInfo.FarInfo, err = composeFarInfo(far, conn.n3Address.To4(), sFarInfo.FarInfo)
			if err != nil {
				return fmt.Errorf("couldn't extract FAR info: %s", err.Error())
			}
			session.UpdateFar(farid, sFarInfo.FarInfo)
			if err := bpfObjects.UpdateFar(sFarInfo.GlobalId, sFarInfo.FarInfo); err != nil {
				return fmt.Errorf("can't update FAR: %s", err.Error())
			}
		}

		for _, far := range msg.RemoveFAR {
			farid, _ := far.FARID()
			sFarInfo := session.RemoveFar(farid)
			if err := bpfObjects.DeleteFar(sFarInfo.GlobalId); err != nil {
				return fmt.Errorf("can't remove FAR: %s", err.Error())
			}
		}

		for _, qer := range msg.CreateQER {
			qerInfo := ebpf.QerInfo{}
			qerId, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing")
			}
			updateQer(&qerInfo, qer)
			if internalId, err := bpfObjects.NewQer(qerInfo); err == nil {
				session.NewQer(qerId, internalId, qerInfo)
			} else {
				return fmt.Errorf("can't put QER: %s", err.Error())
			}
		}

		for _, qer := range msg.UpdateQER {
			qerId, err := qer.QERID() // Probably will be used as ebpf map key
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}
			sQerInfo := session.GetQer(qerId)
			updateQer(&sQerInfo.QerInfo, qer)
			session.UpdateQer(qerId, sQerInfo.QerInfo)
			if err := bpfObjects.UpdateQer(sQerInfo.GlobalId, sQerInfo.QerInfo); err != nil {
				return fmt.Errorf("can't update QER: %s", err.Error())
			}
		}

		for _, qer := range msg.RemoveQER {
			qerId, err := qer.QERID()
			if err != nil {
				return fmt.Errorf("QER ID missing: %s", err.Error())
			}
			sQerInfo := session.RemoveQer(qerId)
			if err := bpfObjects.DeleteQer(sQerInfo.GlobalId); err != nil {
				return fmt.Errorf("can't remove QER: %s", err.Error())
			}
		}

		for _, pdr := range msg.CreatePDR {
			// PDR should be created last, because we need to reference FARs and QERs global id
			pdrId, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := SPDRInfo{PdrID: uint32(pdrId)}

			if err := pdrContext.ExtractPDR(pdr, &spdrInfo); err == nil {
				session.PutPDR(spdrInfo.PdrID, spdrInfo)
				applyPDR(spdrInfo, bpfObjects)
				createdPDRs = append(createdPDRs, spdrInfo)
			} else {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}
		}

		for _, pdr := range msg.UpdatePDR {
			pdrId, err := pdr.PDRID()
			if err != nil {
				return fmt.Errorf("PDR ID missing: %s", err.Error())
			}

			spdrInfo := session.GetPDR(pdrId)
			if err := pdrContext.ExtractPDR(pdr, &spdrInfo); err == nil {
				session.PutPDR(uint32(pdrId), spdrInfo)
				applyPDR(spdrInfo, bpfObjects)
			} else {
				return fmt.Errorf("couldn't extract PDR info: %s", err.Error())
			}
		}

		for _, pdr := range msg.RemovePDR {
			pdrId, _ := pdr.PDRID()
			if _, ok := session.PDRs[uint32(pdrId)]; ok {
				sPDRInfo := session.RemovePDR(uint32(pdrId))

				if err := pdrContext.deletePDR(sPDRInfo, bpfObjects); err != nil {
					return fmt.Errorf("couldn't delete PDR: %s", err.Error())
				}
			}
		}
		logger.UpfLog.Infof("Session modification successful")
		return nil
	}()
	if err != nil {
		logger.UpfLog.Infof("Rejecting Session Modification Request from: %s (failed to apply rules)", err)
		return message.NewSessionModificationResponse(0, 0, session.RemoteSEID, msg.Sequence(), 0, newIeNodeID(conn.nodeId), ie.NewCause(ie.CauseRuleCreationModificationFailure)), nil
	}

	association.Sessions[msg.SEID()] = session

	additionalIEs := []*ie.IE{
		ie.NewCause(ie.CauseRequestAccepted),
		newIeNodeID(conn.nodeId),
	}

	pdrIEs := processCreatedPDRs(createdPDRs, conn.n3Address)
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
		logger.UpfLog.Infof("Unknown error: %s", err.Error())
		return ie.NewCause(ie.CauseRequestRejected)
	}
}

func validateRequest(nodeId *ie.IE, cpfseid *ie.IE) (fseid *ie.FSEIDFields, err error) {
	if nodeId == nil || cpfseid == nil {
		return nil, errMandatoryIeMissing
	}

	_, err = nodeId.NodeID()
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

func composeFarInfo(far *ie.IE, localIp net.IP, farInfo ebpf.FarInfo) (ebpf.FarInfo, error) {
	farInfo.LocalIP = binary.LittleEndian.Uint32(localIp)
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
			logger.UpfLog.Infof("WARN: No OuterHeaderCreation")
		} else {
			outerHeaderCreation, _ := forward[outerHeaderCreationIndex].OuterHeaderCreation()
			farInfo.OuterHeaderCreation = uint8(outerHeaderCreation.OuterHeaderCreationDescription >> 8)
			farInfo.Teid = outerHeaderCreation.TEID
			if outerHeaderCreation.HasIPv4() {
				farInfo.RemoteIP = binary.LittleEndian.Uint32(outerHeaderCreation.IPv4Address)
			}
			if outerHeaderCreation.HasIPv6() {
				logger.UpfLog.Infof("WARN: IPv6 not supported yet, ignoring")
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
		qerInfo.MaxBitrateDL = uint32(maxBitrateDL) * 1000
	}
	maxBitrateUL, err := qer.MBRUL()
	if err == nil {
		qerInfo.MaxBitrateUL = uint32(maxBitrateUL) * 1000
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
