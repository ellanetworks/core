// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
)

func SendPfcpSessionModifyReq(smContext *context.SMContext, pfcpParam *pfcpParam) error {
	defaultPath := smContext.Tunnel.DataPathPool.GetDefaultPath()
	ANUPF := defaultPath.FirstDPNode
	status, err := pfcp.SendPfcpSessionModificationRequest(ANUPF.UPF.NodeID, smContext, pfcpParam.pdrList, pfcpParam.farList, pfcpParam.barList, pfcpParam.qerList)
	if err != nil {
		logger.SmfLog.Warnf("Failed to send PFCP session modification request: %+v", err)
	}

	switch *status {
	case context.SessionUpdateSuccess:
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, PFCP Session Update Success")

	case context.SessionUpdateFailed:
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, PFCP Session Update Failed")
		fallthrough
	case context.SessionUpdateTimeout:
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, PFCP Session Modification Timeout")
		return fmt.Errorf("pfcp session modification failed")
	}

	return nil
}

func SendPfcpSessionReleaseReq(smContext *context.SMContext) error {
	// release UPF data tunnel
	status, ok := releaseTunnel(smContext)
	if !ok {
		logger.SmfLog.Warnf("Failed to release UPF data tunnel")
	}

	switch *status {
	case context.SessionReleaseSuccess:
		smContext.SubCtxLog.Debugln("PDUSessionSMContextUpdate, PFCP Session Release Success")
		return nil
	case context.SessionReleaseTimeout:
		smContext.SubCtxLog.Error("PDUSessionSMContextUpdate, PFCP Session Release Failed")
		return fmt.Errorf("pfcp session release timeout")
	case context.SessionReleaseFailed:
		smContext.SubCtxLog.Error("PDUSessionSMContextUpdate, PFCP Session Release Failed")
		return fmt.Errorf("pfcp session release failed")
	}
	return nil
}
