// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	ctx "context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
)

func ReleaseSmContext(smContextRef string, ctext ctx.Context) error {
	ctxt := context.GetSMContext(smContextRef)
	if ctxt == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}
	err := producer.HandlePDUSessionSMContextRelease(ctxt, ctext)
	if err != nil {
		return fmt.Errorf("error releasing pdu session: %v ", err.Error())
	}
	return nil
}

func UpdateSmContext(smContextRef string, updateSmContextRequest models.UpdateSmContextRequest, ctext ctx.Context) (*models.UpdateSmContextResponse, error) {
	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	if updateSmContextRequest.JSONData == nil {
		return nil, fmt.Errorf("update request is missing JSONData")
	}

	smContext := context.GetSMContext(smContextRef)

	rsp, err := producer.HandlePDUSessionSMContextUpdate(updateSmContextRequest, smContext, ctext)
	if err != nil {
		return rsp, fmt.Errorf("error updating pdu session: %v ", err.Error())
	}
	if rsp == nil {
		return nil, fmt.Errorf("response is nil")
	}
	return rsp, nil
}
