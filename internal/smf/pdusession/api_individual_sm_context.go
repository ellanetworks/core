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

func ReleaseSmContext(smContextRef string) error {
	ctxt := context.GetSMContext(smContextRef)
	if ctxt == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}
	err := producer.HandlePDUSessionSMContextRelease(ctxt)
	ctxt.ChangeState(context.SmStateInit)
	if err != nil {
		return fmt.Errorf("error releasing pdu session: %v ", err.Error())
	}
	logger.SmfLog.Infof("SM Context released successfully: %s", smContextRef)
	return nil
}

func UpdateSmContext(smContextRef string, updateSmContextRequest models.UpdateSmContextRequest) (*models.UpdateSmContextResponse, error) {
	if smContextRef == "" {
		return nil, errors.New("SM Context reference is missing")
	}

	if updateSmContextRequest.JsonData == nil {
		return nil, errors.New("update request is missing JsonData")
	}

	smContext := context.GetSMContext(smContextRef)

	rsp, err := producer.HandlePDUSessionSMContextUpdate(updateSmContextRequest, smContext)
	if err != nil {
		return rsp, fmt.Errorf("error updating pdu session: %v ", err.Error())
	}
	smContext.ChangeState(context.SmStateActive)
	if rsp == nil {
		return nil, errors.New("unexpected error during SM Context update")
	}
	return rsp, nil
}
