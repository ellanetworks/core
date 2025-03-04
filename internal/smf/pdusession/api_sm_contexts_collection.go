// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
)

func SessionCreateInit(req models.PostSmContextsRequest) *context.SMContext {
	createData := req.JsonData
	if smCtxtRef, err := context.ResolveRef(createData.Supi, createData.PduSessionId); err == nil {
		err := producer.HandlePduSessionContextReplacement(smCtxtRef)
		if err != nil {
			logger.SmfLog.Warnf("Failed to replace existing context")
		}
	}
	ctxt := context.NewSMContext(createData.Supi, createData.PduSessionId)
	return ctxt
}

func HandleStateInitEventPduSessCreate(request models.PostSmContextsRequest, smContext *context.SMContext) (context.SMContextState, *models.PostSmContextsResponse, error) {
	rsp, err := producer.HandlePDUSessionSMContextCreate(request, smContext)
	if err != nil {
		return context.SmStateInit, rsp, err
	}
	return context.SmStatePfcpCreatePending, rsp, nil
}

func HandleStatePfcpCreatePendingEventPfcpSessCreate(smCtxt *context.SMContext) (context.SMContextState, error) {
	responseStatus := producer.SendPFCPRules(smCtxt)
	switch responseStatus {
	case context.SessionEstablishSuccess:
		smCtxt.SubFsmLog.Infof("pfcp session establish response success")
		return context.SmStateN1N2TransferPending, nil
	case context.SessionEstablishFailed:
		fallthrough
	default:
		smCtxt.SubFsmLog.Errorf("pfcp session establish response failure")
		return context.SmStatePfcpCreatePending, fmt.Errorf("pfcp establishment failure")
	}
}

func HandleStateN1N2TransferPendingEventN1N2Transfer(smCtxt *context.SMContext) (context.SMContextState, error) {
	if err := producer.SendPduSessN1N2Transfer(smCtxt, true); err != nil {
		smCtxt.SubFsmLog.Errorf("N1N2 transfer failure error, %v ", err.Error())
		return context.SmStateN1N2TransferPending, fmt.Errorf("N1N2 Transfer failure error, %v ", err.Error())
	}
	return context.SmStateActive, nil
}

func HandleStatePfcpCreatePendingEventPfcpSessCreateFailure(smCtxt *context.SMContext) (context.SMContextState, error) {
	if err := producer.SendPduSessN1N2Transfer(smCtxt, false); err != nil {
		smCtxt.SubFsmLog.Errorf("N1N2 transfer failure error, %v ", err.Error())
		return context.SmStateN1N2TransferPending, fmt.Errorf("N1N2 Transfer failure error, %v ", err.Error())
	}
	return context.SmStateInit, nil
}

func CreateSmContext(request models.PostSmContextsRequest) (*models.PostSmContextsResponse, *models.PostSmContextsErrorResponse, error) {
	// Ensure request data is present
	if request.JsonData == nil {
		errResponse := &models.PostSmContextsErrorResponse{}
		return nil, errResponse, fmt.Errorf("missing JsonData in request")
	}
	smContext := SessionCreateInit(request)
	nextState, rsp, err := HandleStateInitEventPduSessCreate(request, smContext)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SM Context: %v", err)
	}
	smContext.ChangeState(nextState)

	nextState, err = HandleStatePfcpCreatePendingEventPfcpSessCreate(smContext)
	smContext.ChangeState(nextState)
	if err != nil {
		if smContext != nil && smContext.SMContextState == context.SmStatePfcpCreatePending {
			go func() {
				nextState, err := HandleStatePfcpCreatePendingEventPfcpSessCreateFailure(smContext)
				if err != nil {
					logger.SmfLog.Errorf("error processing state machine transaction")
				}
				smContext.ChangeState(nextState)
			}()
		}
	} else {
		go func() {
			nextState, err := HandleStateN1N2TransferPendingEventN1N2Transfer(smContext)
			smContext.ChangeState(nextState)
			if err != nil {
				logger.SmfLog.Errorf("error processing state machine transaction")
			}
		}()
	}

	return rsp, nil, nil
}
