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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf")

func CreateSmContext(ctx ctxt.Context, request models.PostSmContextsRequest) (string, *models.PostSmContextsErrorResponse, error) {
	ctx, span := tracer.Start(ctx, "SMF Create SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.supi", request.JSONData.Supi),
	)
	if request.JSONData == nil {
		errResponse := &models.PostSmContextsErrorResponse{}
		return "", errResponse, fmt.Errorf("missing JSONData in request")
	}

	createData := request.JSONData
	smCtxtRef, err := context.ResolveRef(createData.Supi, createData.PduSessionID)
	if err == nil {
		err := producer.HandlePduSessionContextReplacement(ctx, smCtxtRef)
		if err != nil {
			return "", nil, fmt.Errorf("failed to replace existing context")
		}
	}

	smContext := context.NewSMContext(createData.Supi, createData.PduSessionID)

	location, errRsp, err := producer.HandlePDUSessionSMContextCreate(ctx, request, smContext)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	if errRsp != nil {
		return "", errRsp, nil
	}

	err = producer.SendPFCPRules(ctx, smContext)
	if err != nil {
		if smContext != nil {
			go func() {
				err := producer.SendPduSessN1N2Transfer(ctx, smContext, false)
				if err != nil {
					logger.SmfLog.Error("error transferring n1 n2", zap.Error(err))
				}
			}()
		}
		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	go func() {
		err := producer.SendPduSessN1N2Transfer(ctx, smContext, true)
		if err != nil {
			logger.SmfLog.Error("error transferring n1 n2", zap.Error(err))
		}
	}()
	return location, nil, nil
}
