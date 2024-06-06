package upf

import (
	"time"

	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
)

const (
	maxHeartbeatRetry        = 3  // sec
	maxHeartbeatInterval     = 10 // sec
	maxUpfProbeRetryInterval = 10 // sec
)

func InitPfcpHeartbeatRequest(userplane *context.UserPlaneInformation) {
	// Iterate through all UPFs and send heartbeat to active UPFs
	for {
		time.Sleep(maxHeartbeatInterval * time.Second)
		for _, upf := range userplane.UPFs {
			upf.UPF.UpfLock.Lock()
			if (upf.UPF.UPFStatus == context.AssociatedSetUpSuccess) && upf.UPF.NHeartBeat < maxHeartbeatRetry {
				err := message.SendHeartbeatRequest(upf.NodeID, upf.Port) // needs lock in sync rsp(adapter mode)
				if err != nil {
					logger.PfcpLog.Errorf("send pfcp heartbeat request failed: %v for UPF[%v, %v]: ", err, upf.NodeID, upf.NodeID.ResolveNodeIdToIp())
				} else {
					upf.UPF.NHeartBeat++
				}
			} else if upf.UPF.NHeartBeat == maxHeartbeatRetry {
				logger.PfcpLog.Errorf("pfcp heartbeat failure for UPF: [%v]", upf.NodeID)
				upf.UPF.UPFStatus = context.NotAssociated
			}

			upf.UPF.UpfLock.Unlock()
		}
	}
}

func ProbeInactiveUpfs(upfs *context.UserPlaneInformation) {
	// Iterate through all UPFs and send PFCP request to inactive UPFs
	for {
		time.Sleep(maxUpfProbeRetryInterval * time.Second)
		for _, upf := range upfs.UPFs {
			upf.UPF.UpfLock.Lock()
			if upf.UPF.UPFStatus == context.NotAssociated {
				message.SendPfcpAssociationSetupRequest(upf.NodeID, upf.Port)
			}
			upf.UPF.UpfLock.Unlock()
		}
	}
}
