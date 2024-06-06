package message

import (
	"fmt"
	"net"

	"github.com/omec-project/pfcp"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
)

// BuildPfcpHeartbeatRequest shall trigger hearbeat request to all Attached UPFs
func BuildPfcpHeartbeatRequest() (pfcp.HeartbeatRequest, error) {
	msg := pfcp.HeartbeatRequest{}

	msg.RecoveryTimeStamp = &pfcpType.RecoveryTimeStamp{
		RecoveryTimeStamp: udp.ServerStartTime,
	}

	return msg, nil
}

func BuildPfcpAssociationSetupRequest() (pfcp.PFCPAssociationSetupRequest, error) {
	msg := pfcp.PFCPAssociationSetupRequest{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	msg.RecoveryTimeStamp = &pfcpType.RecoveryTimeStamp{
		RecoveryTimeStamp: udp.ServerStartTime,
	}

	msg.CPFunctionFeatures = &pfcpType.CPFunctionFeatures{
		SupportedFeatures: 0,
	}

	return msg, nil
}

func BuildPfcpAssociationSetupResponse(cause pfcpType.Cause) (pfcp.PFCPAssociationSetupResponse, error) {
	msg := pfcp.PFCPAssociationSetupResponse{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	msg.Cause = &cause

	msg.RecoveryTimeStamp = &pfcpType.RecoveryTimeStamp{
		RecoveryTimeStamp: udp.ServerStartTime,
	}

	msg.CPFunctionFeatures = &pfcpType.CPFunctionFeatures{
		SupportedFeatures: 0,
	}

	return msg, nil
}

func BuildPfcpAssociationReleaseRequest() (pfcp.PFCPAssociationReleaseRequest, error) {
	msg := pfcp.PFCPAssociationReleaseRequest{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	return msg, nil
}

func BuildPfcpAssociationReleaseResponse(cause pfcpType.Cause) (pfcp.PFCPAssociationReleaseResponse, error) {
	msg := pfcp.PFCPAssociationReleaseResponse{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	msg.Cause = &cause

	return msg, nil
}

func pdrToCreatePDR(pdr *context.PDR) *pfcp.CreatePDR {
	createPDR := new(pfcp.CreatePDR)

	createPDR.PDRID = new(pfcpType.PacketDetectionRuleID)
	createPDR.PDRID.RuleId = pdr.PDRID

	createPDR.Precedence = new(pfcpType.Precedence)
	createPDR.Precedence.PrecedenceValue = pdr.Precedence

	createPDR.PDI = &pfcp.PDI{
		SourceInterface: &pdr.PDI.SourceInterface,
		LocalFTEID:      pdr.PDI.LocalFTeid,
		NetworkInstance: &pdr.PDI.NetworkInstance,
		UEIPAddress:     pdr.PDI.UEIPAddress,
	}

	if pdr.PDI.ApplicationID != "" {
		createPDR.PDI.ApplicationID = &pfcpType.ApplicationID{
			ApplicationIdentifier: []byte(pdr.PDI.ApplicationID),
		}
	}

	if pdr.PDI.SDFFilter != nil {
		createPDR.PDI.SDFFilter = pdr.PDI.SDFFilter
	}

	createPDR.OuterHeaderRemoval = pdr.OuterHeaderRemoval

	createPDR.FARID = &pfcpType.FARID{
		FarIdValue: pdr.FAR.FARID,
	}

	for _, qer := range pdr.QER {
		if qer != nil {
			createPDR.QERID = append(createPDR.QERID, &pfcpType.QERID{
				QERID: qer.QERID,
			})
		}
	}

	return createPDR
}

func farToCreateFAR(far *context.FAR) *pfcp.CreateFAR {
	createFAR := new(pfcp.CreateFAR)

	createFAR.FARID = new(pfcpType.FARID)
	createFAR.FARID.FarIdValue = far.FARID

	createFAR.ApplyAction = new(pfcpType.ApplyAction)
	createFAR.ApplyAction.Dupl = far.ApplyAction.Dupl
	createFAR.ApplyAction.Nocp = far.ApplyAction.Nocp
	createFAR.ApplyAction.Buff = far.ApplyAction.Buff
	createFAR.ApplyAction.Forw = far.ApplyAction.Forw
	createFAR.ApplyAction.Drop = far.ApplyAction.Drop

	if far.BAR != nil {
		createFAR.BARID = new(pfcpType.BARID)
		createFAR.BARID.BarIdValue = far.BAR.BARID
	}

	if far.ForwardingParameters != nil {
		createFAR.ForwardingParameters = new(pfcp.ForwardingParametersIEInFAR)
		createFAR.ForwardingParameters.DestinationInterface = &far.ForwardingParameters.DestinationInterface
		createFAR.ForwardingParameters.NetworkInstance = &far.ForwardingParameters.NetworkInstance
		createFAR.ForwardingParameters.OuterHeaderCreation = far.ForwardingParameters.OuterHeaderCreation
		if far.ForwardingParameters.ForwardingPolicyID != "" {
			createFAR.ForwardingParameters.ForwardingPolicy = new(pfcpType.ForwardingPolicy)
			createFAR.ForwardingParameters.ForwardingPolicy.ForwardingPolicyIdentifierLength = uint8(len(far.ForwardingParameters.ForwardingPolicyID))
			createFAR.ForwardingParameters.ForwardingPolicy.ForwardingPolicyIdentifier = []byte(far.ForwardingParameters.ForwardingPolicyID)
		}
	}

	return createFAR
}

func barToCreateBAR(bar *context.BAR) *pfcp.CreateBAR {
	createBAR := new(pfcp.CreateBAR)

	createBAR.BARID = new(pfcpType.BARID)
	createBAR.BARID.BarIdValue = bar.BARID

	createBAR.DownlinkDataNotificationDelay = new(pfcpType.DownlinkDataNotificationDelay)

	// createBAR.SuggestedBufferingPacketsCount = new(pfcpType.SuggestedBufferingPacketsCount)

	return createBAR
}

func qerToCreateQER(qer *context.QER) *pfcp.CreateQER {
	createQER := new(pfcp.CreateQER)

	createQER.QERID = new(pfcpType.QERID)
	createQER.QERID.QERID = qer.QERID
	createQER.GateStatus = qer.GateStatus

	createQER.QoSFlowIdentifier = &qer.QFI
	createQER.MaximumBitrate = qer.MBR
	createQER.GuaranteedBitrate = qer.GBR

	return createQER
}

func pdrToUpdatePDR(pdr *context.PDR) *pfcp.UpdatePDR {
	updatePDR := new(pfcp.UpdatePDR)

	updatePDR.PDRID = new(pfcpType.PacketDetectionRuleID)
	updatePDR.PDRID.RuleId = pdr.PDRID

	updatePDR.Precedence = new(pfcpType.Precedence)
	updatePDR.Precedence.PrecedenceValue = pdr.Precedence

	updatePDR.PDI = &pfcp.PDI{
		SourceInterface: &pdr.PDI.SourceInterface,
		LocalFTEID:      pdr.PDI.LocalFTeid,
		NetworkInstance: &pdr.PDI.NetworkInstance,
		UEIPAddress:     pdr.PDI.UEIPAddress,
	}

	if pdr.PDI.ApplicationID != "" {
		updatePDR.PDI.ApplicationID = &pfcpType.ApplicationID{
			ApplicationIdentifier: []byte(pdr.PDI.ApplicationID),
		}
	}

	if pdr.PDI.SDFFilter != nil {
		updatePDR.PDI.SDFFilter = pdr.PDI.SDFFilter
	}

	updatePDR.OuterHeaderRemoval = pdr.OuterHeaderRemoval

	updatePDR.FARID = &pfcpType.FARID{
		FarIdValue: pdr.FAR.FARID,
	}

	for _, qer := range pdr.QER {
		if qer != nil {
			updatePDR.QERID = append(updatePDR.QERID, &pfcpType.QERID{
				QERID: qer.QERID,
			})
		}
	}

	return updatePDR
}

func farToUpdateFAR(far *context.FAR) *pfcp.UpdateFAR {
	updateFAR := new(pfcp.UpdateFAR)

	updateFAR.FARID = new(pfcpType.FARID)
	updateFAR.FARID.FarIdValue = far.FARID

	if far.BAR != nil {
		updateFAR.BARID = new(pfcpType.BARID)
		updateFAR.BARID.BarIdValue = far.BAR.BARID
	}

	updateFAR.ApplyAction = new(pfcpType.ApplyAction)
	updateFAR.ApplyAction.Forw = far.ApplyAction.Forw
	updateFAR.ApplyAction.Buff = far.ApplyAction.Buff
	updateFAR.ApplyAction.Nocp = far.ApplyAction.Nocp
	updateFAR.ApplyAction.Dupl = far.ApplyAction.Dupl
	updateFAR.ApplyAction.Drop = far.ApplyAction.Drop

	if far.ForwardingParameters != nil {
		updateFAR.UpdateForwardingParameters = new(pfcp.UpdateForwardingParametersIEInFAR)
		updateFAR.UpdateForwardingParameters.DestinationInterface = &far.ForwardingParameters.DestinationInterface
		updateFAR.UpdateForwardingParameters.NetworkInstance = &far.ForwardingParameters.NetworkInstance
		updateFAR.UpdateForwardingParameters.OuterHeaderCreation = far.ForwardingParameters.OuterHeaderCreation
		if far.ForwardingParameters.PFCPSMReqFlags != nil {
			updateFAR.UpdateForwardingParameters.PFCPSMReqFlags = far.ForwardingParameters.PFCPSMReqFlags
			// reset original far sndem flag
			far.ForwardingParameters.PFCPSMReqFlags = nil
		}

		if far.ForwardingParameters.ForwardingPolicyID != "" {
			updateFAR.UpdateForwardingParameters.ForwardingPolicy = new(pfcpType.ForwardingPolicy)
			updateFAR.UpdateForwardingParameters.ForwardingPolicy.ForwardingPolicyIdentifierLength = uint8(len(far.ForwardingParameters.ForwardingPolicyID))
			updateFAR.UpdateForwardingParameters.ForwardingPolicy.ForwardingPolicyIdentifier = []byte(far.ForwardingParameters.ForwardingPolicyID)
		}
	}

	return updateFAR
}

func BuildPfcpSessionEstablishmentRequest(
	upNodeID pfcpType.NodeID,
	smContext *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) (pfcp.PFCPSessionEstablishmentRequest, error) {
	msg := pfcp.PFCPSessionEstablishmentRequest{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	isv4 := context.SMF_Self().CPNodeID.NodeIdType == 0
	nodeIDtoIP := upNodeID.ResolveNodeIdToIp().String()

	localSEID := smContext.PFCPContext[nodeIDtoIP].LocalSEID
	fmt.Printf("in BuildPfcpSessionEstablishmentRequest localSEID %v\n", localSEID)
	msg.CPFSEID = &pfcpType.FSEID{
		V4:          isv4,
		V6:          !isv4,
		Seid:        localSEID,
		Ipv4Address: context.SMF_Self().CPNodeID.NodeIdValue,
	}

	msg.CreatePDR = make([]*pfcp.CreatePDR, 0)
	msg.CreateFAR = make([]*pfcp.CreateFAR, 0)

	for _, pdr := range pdrList {
		if pdr.State == context.RULE_INITIAL {
			msg.CreatePDR = append(msg.CreatePDR, pdrToCreatePDR(pdr))
		}
		pdr.State = context.RULE_CREATE
	}

	for _, far := range farList {
		if far.State == context.RULE_INITIAL {
			msg.CreateFAR = append(msg.CreateFAR, farToCreateFAR(far))
		}
		far.State = context.RULE_CREATE
	}

	for _, bar := range barList {
		if bar.State == context.RULE_INITIAL {
			msg.CreateBAR = append(msg.CreateBAR, barToCreateBAR(bar))
		}
		bar.State = context.RULE_CREATE
	}

	// QER maybe redundant, so we needs properly needs

	qerMap := make(map[uint32]*context.QER)
	for _, qer := range qerList {
		qerMap[qer.QERID] = qer
	}
	for _, filteredQER := range qerMap {
		if filteredQER.State == context.RULE_INITIAL {
			msg.CreateQER = append(msg.CreateQER, qerToCreateQER(filteredQER))
		}
		filteredQER.State = context.RULE_CREATE
	}

	msg.PDNType = &pfcpType.PDNType{
		PdnType: pfcpType.PDNTypeIpv4,
	}

	// for _, far := range msg.CreateFAR {
	// 	printCreateFAR(far)
	// }

	return msg, nil
}

func BuildPfcpSessionEstablishmentResponse() (pfcp.PFCPSessionEstablishmentResponse, error) {
	msg := pfcp.PFCPSessionEstablishmentResponse{}

	msg.NodeID = &context.SMF_Self().CPNodeID

	msg.Cause = &pfcpType.Cause{
		CauseValue: pfcpType.CauseRequestAccepted,
	}

	msg.OffendingIE = &pfcpType.OffendingIE{
		TypeOfOffendingIe: 12345,
	}

	msg.UPFSEID = &pfcpType.FSEID{
		V4:          true,
		V6:          false, //;
		Seid:        123456789123456789,
		Ipv4Address: net.ParseIP("192.168.1.1").To4(),
	}

	msg.CreatedPDR = &pfcp.CreatedPDR{
		PDRID: &pfcpType.PacketDetectionRuleID{
			RuleId: 256,
		},
		LocalFTEID: &pfcpType.FTEID{
			Chid:        false,
			Ch:          false,
			V6:          false,
			V4:          true,
			Teid:        12345,
			Ipv4Address: net.ParseIP("192.168.1.1").To4(),
		},
	}

	return msg, nil
}

// TODO: Replace dummy value in PFCP message
func BuildPfcpSessionModificationRequest(
	upNodeID pfcpType.NodeID,
	smContext *context.SMContext,
	pdrList []*context.PDR,
	farList []*context.FAR,
	barList []*context.BAR,
	qerList []*context.QER,
) (pfcp.PFCPSessionModificationRequest, error) {
	msg := pfcp.PFCPSessionModificationRequest{}

	msg.UpdatePDR = make([]*pfcp.UpdatePDR, 0, 2)
	msg.UpdateFAR = make([]*pfcp.UpdateFAR, 0, 2)

	nodeIDtoIP := upNodeID.ResolveNodeIdToIp().String()

	localSEID := smContext.PFCPContext[nodeIDtoIP].LocalSEID

	msg.CPFSEID = &pfcpType.FSEID{
		V4:          true,
		V6:          false,
		Seid:        localSEID,
		Ipv4Address: context.SMF_Self().CPNodeID.NodeIdValue,
	}

	for _, pdr := range pdrList {
		switch pdr.State {
		case context.RULE_INITIAL:
			msg.CreatePDR = append(msg.CreatePDR, pdrToCreatePDR(pdr))
		case context.RULE_UPDATE:
			msg.UpdatePDR = append(msg.UpdatePDR, pdrToUpdatePDR(pdr))
		case context.RULE_REMOVE:
			msg.RemovePDR = append(msg.RemovePDR, &pfcp.RemovePDR{
				PDRID: &pfcpType.PacketDetectionRuleID{
					RuleId: pdr.PDRID,
				},
			})
		}
		pdr.State = context.RULE_CREATE
	}

	for _, far := range farList {
		switch far.State {
		case context.RULE_INITIAL:
			msg.CreateFAR = append(msg.CreateFAR, farToCreateFAR(far))
		case context.RULE_UPDATE:
			msg.UpdateFAR = append(msg.UpdateFAR, farToUpdateFAR(far))
		case context.RULE_REMOVE:
			msg.RemoveFAR = append(msg.RemoveFAR, &pfcp.RemoveFAR{
				FARID: &pfcpType.FARID{
					FarIdValue: far.FARID,
				},
			})
		}
		far.State = context.RULE_CREATE
	}

	for _, bar := range barList {
		switch bar.State {
		case context.RULE_INITIAL:
			msg.CreateBAR = append(msg.CreateBAR, barToCreateBAR(bar))
		}
	}

	for _, qer := range qerList {
		switch qer.State {
		case context.RULE_INITIAL:
			msg.CreateQER = append(msg.CreateQER, qerToCreateQER(qer))
		}
		qer.State = context.RULE_CREATE
	}

	return msg, nil
}

// TODO: Replace dummy value in PFCP message
func BuildPfcpSessionModificationResponse() (pfcp.PFCPSessionModificationResponse, error) {
	msg := pfcp.PFCPSessionModificationResponse{}

	msg.Cause = &pfcpType.Cause{
		CauseValue: pfcpType.CauseRequestAccepted,
	}

	msg.OffendingIE = &pfcpType.OffendingIE{
		TypeOfOffendingIe: 12345,
	}

	msg.CreatedPDR = &pfcp.CreatedPDR{
		PDRID: &pfcpType.PacketDetectionRuleID{
			RuleId: 256,
		},
		LocalFTEID: &pfcpType.FTEID{
			Chid:        false,
			Ch:          false,
			V6:          false,
			V4:          true,
			Teid:        12345,
			Ipv4Address: net.ParseIP("192.168.1.1").To4(),
		},
	}

	return msg, nil
}

func BuildPfcpSessionDeletionRequest(
	upNodeID pfcpType.NodeID,
	smContext *context.SMContext,
) (pfcp.PFCPSessionDeletionRequest, error) {
	msg := pfcp.PFCPSessionDeletionRequest{}

	nodeIDtoIP := upNodeID.ResolveNodeIdToIp().String()

	localSEID := smContext.PFCPContext[nodeIDtoIP].LocalSEID

	msg.CPFSEID = &pfcpType.FSEID{
		V4:          true,
		V6:          false,
		Seid:        localSEID,
		Ipv4Address: context.SMF_Self().CPNodeID.NodeIdValue,
	}
	return msg, nil
}

// TODO: Replace dummy value in PFCP message
func BuildPfcpSessionDeletionResponse() (pfcp.PFCPSessionDeletionResponse, error) {
	msg := pfcp.PFCPSessionDeletionResponse{}

	msg.Cause = &pfcpType.Cause{
		CauseValue: pfcpType.CauseRequestAccepted,
	}

	msg.OffendingIE = &pfcpType.OffendingIE{
		TypeOfOffendingIe: 12345,
	}

	return msg, nil
}

func BuildPfcpSessionReportResponse(cause pfcpType.Cause, pfcpSRflag pfcpType.PFCPSRRspFlags) (pfcp.PFCPSessionReportResponse, error) {
	msg := pfcp.PFCPSessionReportResponse{}

	msg.Cause = &cause

	if pfcpSRflag.Drobu {
		msg.SxSRRspFlags = &pfcpType.PFCPSRRspFlags{Drobu: true}
	}

	return msg, nil
}
