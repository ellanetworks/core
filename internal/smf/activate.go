// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ActivateSmContext re-activates an existing PDU session (e.g. after idle-mode paging).
// It returns the N2 PDUSessionResourceSetupRequest transfer.
func (s *SMF) ActivateSmContext(ctx context.Context, smContextRef string) ([]byte, error) {
	_, span := tracer.Start(ctx, "SMF activate session",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	n2Buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Buf, nil
}
