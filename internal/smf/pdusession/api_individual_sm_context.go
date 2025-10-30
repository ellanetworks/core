// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func ReleaseSmContext(ctx ctxt.Context, smContextRef string) error {
	logger.SmfLog.Debug("Releasing SM Context", zap.String("smContextRef", smContextRef))
	ctx, span := tracer.Start(ctx, "SMF Release SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)
	ctxt := context.GetSMContext(smContextRef)
	if ctxt == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}
	err := producer.HandlePDUSessionSMContextRelease(ctx, ctxt)
	if err != nil {
		return fmt.Errorf("error releasing pdu session: %v ", err.Error())
	}
	return nil
}

func UpdateSmContext(ctx ctxt.Context, smContextRef string, updateSmContextRequest models.UpdateSmContextRequest) (*models.UpdateSmContextResponse, error) {
	logger.SmfLog.Debug("Updating SM Context", zap.String("smContextRef", smContextRef))
	ctx, span := tracer.Start(ctx, "SMF Update SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)
	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	if updateSmContextRequest.JSONData == nil {
		return nil, fmt.Errorf("update request is missing JSONData")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	rsp, err := producer.HandlePDUSessionSMContextUpdate(ctx, updateSmContextRequest, smContext)
	if err != nil {
		return rsp, fmt.Errorf("error updating pdu session: %v ", err.Error())
	}

	if rsp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	go func() {
		err := producer.SendPduSessN1N2Transfer(ctx, smContext, true)
		if err != nil {
			logger.SmfLog.Error("error transferring n1 n2", zap.Error(err))
		}
	}()

	return rsp, nil
}
