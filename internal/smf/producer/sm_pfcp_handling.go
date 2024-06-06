package producer

import (
	"fmt"

	smf_context "github.com/yeastengine/ella/internal/smf/context"
	pfcp_message "github.com/yeastengine/ella/internal/smf/pfcp/message"
)

func SendPfcpSessionModifyReq(smContext *smf_context.SMContext, pfcpParam *pfcpParam) error {
	defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
	ANUPF := defaultPath.FirstDPNode
	pfcp_message.SendPfcpSessionModificationRequest(ANUPF.UPF.NodeID, smContext,
		pfcpParam.pdrList, pfcpParam.farList, pfcpParam.barList, pfcpParam.qerList, ANUPF.UPF.Port)

	PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan

	switch PFCPResponseStatus {
	case smf_context.SessionUpdateSuccess:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Update Success")

	case smf_context.SessionUpdateFailed:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Update Failed")
		fallthrough
	case smf_context.SessionUpdateTimeout:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Modification Timeout")

		err := fmt.Errorf("pfcp modification failure")
		return err
	}

	return nil
}

func SendPfcpSessionReleaseReq(smContext *smf_context.SMContext) error {
	// release UPF data tunnel
	releaseTunnel(smContext)

	PFCPResponseStatus := <-smContext.SBIPFCPCommunicationChan
	switch PFCPResponseStatus {
	case smf_context.SessionReleaseSuccess:
		smContext.SubCtxLog.Traceln("PDUSessionSMContextUpdate, PFCP Session Release Success")
		return nil
	case smf_context.SessionReleaseTimeout:
		smContext.SubCtxLog.Error("PDUSessionSMContextUpdate, PFCP Session Release Failed")
		return fmt.Errorf("pfcp session release timeout")
	case smf_context.SessionReleaseFailed:
		smContext.SubCtxLog.Error("PDUSessionSMContextUpdate, PFCP Session Release Failed")
		return fmt.Errorf("pfcp session release failed")
	}
	return nil
}
