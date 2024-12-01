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
	for {
		time.Sleep(maxHeartbeatInterval * time.Second)
		if userplane == nil {
			continue
		}
		if userplane.UPF == nil {
			continue
		}
		userplane.UPF.UPF.UpfLock.Lock()
		if (userplane.UPF.UPF.UPFStatus == context.AssociatedSetUpSuccess) && userplane.UPF.UPF.NHeartBeat < maxHeartbeatRetry {
			err := message.SendHeartbeatRequest(userplane.UPF.NodeID, userplane.UPF.Port) // needs lock in sync rsp(adapter mode)
			if err != nil {
				logger.PfcpLog.Errorf("send pfcp heartbeat request failed: %v for UPF[%v, %v]: ", err, userplane.UPF.NodeID, userplane.UPF.NodeID.ResolveNodeIdToIp())
			} else {
				userplane.UPF.UPF.NHeartBeat++
			}
		} else if userplane.UPF.UPF.NHeartBeat == maxHeartbeatRetry {
			if userplane.UPF.UPF.UPFStatus == context.AssociatedSetUpSuccess {
				userplane.UPF.UPF.UPFStatus = context.NotAssociated
				logger.PfcpLog.Warnf("did not receive heartbeat response from UPF [%v], set UPF status to NotAssociated", userplane.UPF.NodeID.ResolveNodeIdToIp())
			}
		}

		userplane.UPF.UPF.UpfLock.Unlock()
	}
}

func ProbeInactiveUpfs(upfs *context.UserPlaneInformation) {
	// Iterate through all UPFs and send PFCP request to inactive UPFs
	for {
		time.Sleep(maxUpfProbeRetryInterval * time.Second)
		if upfs == nil {
			continue
		}
		if upfs.UPF == nil {
			continue
		}
		upfs.UPF.UPF.UpfLock.Lock()
		if upfs.UPF.UPF.UPFStatus == context.NotAssociated {
			err := message.SendPfcpAssociationSetupRequest(upfs.UPF.NodeID, upfs.UPF.Port)
			if err != nil {
				logger.PfcpLog.Errorf("send pfcp association setup request failed: %v ", err)
			}
		}
		upfs.UPF.UPF.UpfLock.Unlock()
	}
}
