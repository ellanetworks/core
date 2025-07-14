// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
)

func SendPfcpSessionModifyReq(ctx ctxt.Context, smContext *context.SMContext, pfcpParam *pfcpParam) error {
	dataPath := smContext.Tunnel.DataPath
	ANUPF := dataPath.DPNode
	err := pfcp.SendPfcpSessionModificationRequest(ctx, ANUPF.UPF.NodeID, smContext, pfcpParam.pdrList, pfcpParam.farList, pfcpParam.barList, pfcpParam.qerList)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}
	return nil
}

func SendPfcpSessionReleaseReq(ctx ctxt.Context, smContext *context.SMContext) error {
	err := releaseTunnel(ctx, smContext)
	if err != nil {
		return fmt.Errorf("failed to release tunnel: %v", err)
	}
	return nil
}
