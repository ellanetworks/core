// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
)

func HandleStateActiveEventPduSessRelease(smCtxt *context.SMContext) (context.SMContextState, error) {
	err := producer.HandlePDUSessionSMContextRelease(smCtxt)
	if err != nil {
		return context.SmStateInit, err
	}
	return context.SmStateInit, nil
}

func ReleaseSmContext(smContextRef string) error {
	ctxt := context.GetSMContext(smContextRef)
	nextState, err := HandleStateActiveEventPduSessRelease(ctxt)
	ctxt.ChangeState(nextState)
	if err != nil {
		return fmt.Errorf("error releasing pdu session: %v ", err.Error())
	}
	logger.SmfLog.Infof("SM Context released successfully: %s", smContextRef)
	return nil
}

func HandlePduSessModify(request models.UpdateSmContextRequest, smCtxt *context.SMContext) (context.SMContextState, *models.UpdateSmContextResponse, error) {
	rsp, err := producer.HandlePDUSessionSMContextUpdate(request, smCtxt)
	if err != nil {
		return context.SmStateActive, rsp, fmt.Errorf("error updating pdu session: %v ", err.Error())
	}
	return context.SmStateActive, rsp, nil
}

func UpdateSmContext(smContextRef string, updateSmContextRequest models.UpdateSmContextRequest) (*models.UpdateSmContextResponse, error) {
	logger.SmfLog.Info("Processing Update SM Context Request")

	if smContextRef == "" {
		return nil, errors.New("SM Context reference is missing")
	}

	if updateSmContextRequest.JsonData == nil {
		return nil, errors.New("update request is missing JsonData")
	}

	smContext := context.GetSMContext(smContextRef)

	nextState, rsp, err := HandlePduSessModify(updateSmContextRequest, smContext)
	if err != nil {
		logger.SmfLog.Errorf("error processing state machine transaction")
	}
	smContext.ChangeState(nextState)
	if rsp == nil {
		return nil, errors.New("unexpected error during SM Context update")
	}
	return rsp, nil
}
