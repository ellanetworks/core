package context

import (
	"fmt"

	"github.com/omec-project/pfcp/pfcpType"
)

type PFCPSessionResponseStatus int

const (
	SessionUpdateSuccess PFCPSessionResponseStatus = iota
	SessionUpdateFailed
	SessionReleaseSuccess
	SessionReleaseFailed
	SessionUpdateTimeout
	SessionReleaseTimeout
	SessionEstablishSuccess
	SessionEstablishFailed
	SessionEstablishTimeout
)

type PFCPSessionContext struct {
	PDRs       map[uint16]*PDR
	NodeID     pfcpType.NodeID
	LocalSEID  uint64
	RemoteSEID uint64
}

func (pfcpSessionContext *PFCPSessionContext) String() string {
	str := "\n"
	for pdrID, pdr := range pfcpSessionContext.PDRs {
		str += fmt.Sprintln("PDR ID: ", pdrID)
		str += fmt.Sprintf("PDR: %v\n", pdr)
	}

	str += fmt.Sprintln("Node ID: ", pfcpSessionContext.NodeID.ResolveNodeIdToIp().String())
	str += fmt.Sprintln("LocalSEID: ", pfcpSessionContext.LocalSEID)
	str += fmt.Sprintln("RemoteSEID: ", pfcpSessionContext.RemoteSEID)
	str += "\n"

	return str
}

func (pfcpSessionResponseStatus PFCPSessionResponseStatus) String() string {
	switch pfcpSessionResponseStatus {
	case SessionUpdateSuccess:
		return "SessionUpdateSuccess"
	case SessionUpdateFailed:
		return "SessionUpdateFailed"
	case SessionReleaseSuccess:
		return "SessionReleaseSuccess"
	case SessionReleaseFailed:
		return "SessionReleaseFailed"
	case SessionUpdateTimeout:
		return "SessionUpdateTimeout"
	case SessionReleaseTimeout:
		return "SessionReleaseTimeout"
	default:
		return "Unknown PFCP Session Response Status"
	}
}
