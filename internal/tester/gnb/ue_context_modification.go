// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleUEContextModificationRequest(gnb *GnodeB, value []byte) error {
	req, err := ngap.ParseUEContextModificationRequest(value)
	if err != nil {
		return fmt.Errorf("undecodable UEContextModificationRequest: %w", err)
	}

	fields := []zap.Field{
		zap.Int64("RAN UE NGAP ID", int64(req.RANUENGAPID)),
		zap.Int64("AMF UE NGAP ID", int64(req.AMFUENGAPID)),
	}

	if ambr := req.UEAggregateMaximumBitRate; ambr != nil {
		fields = append(fields, zap.Uint64("UE-AMBR DL", uint64(ambr.DL)), zap.Uint64("UE-AMBR UL", uint64(ambr.UL)))
	}

	logger.GnbLogger.Debug("Received UE Context Modification Request", fields...)

	b, err := (&ngap.UEContextModificationResponse{
		AMFUENGAPID: ngap.Ptr(req.AMFUENGAPID),
		RANUENGAPID: ngap.Ptr(req.RANUENGAPID),
	}).Marshal()
	if err != nil {
		return fmt.Errorf("build UEContextModificationResponse: %w", err)
	}

	return gnb.SendMessage(b, NGAPProcedureUEContextModificationResponse)
}

func (g *GnodeB) WaitForUEContextModificationRequest(timeout time.Duration) (*ngap.UEContextModificationRequest, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcUEContextModification, timeout)
	if err != nil {
		return nil, err
	}

	req, err := ngap.ParseUEContextModificationRequest(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse UE Context Modification Request: %w", err)
	}

	return req, nil
}
