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

func CreateSmContext(request models.PostSmContextsRequest) (string, *models.PostSmContextsErrorResponse, error) {
	if request.JSONData == nil {
		errResponse := &models.PostSmContextsErrorResponse{}
		return "", errResponse, fmt.Errorf("missing JSONData in request")
	}

	createData := request.JSONData
	smCtxtRef, err := context.ResolveRef(createData.Supi, createData.PduSessionID)
	if err == nil {
		err := producer.HandlePduSessionContextReplacement(smCtxtRef)
		if err != nil {
			return "", nil, fmt.Errorf("failed to replace existing context")
		}
	}

	smContext := context.NewSMContext(createData.Supi, createData.PduSessionID)

	location, errRsp, err := producer.HandlePDUSessionSMContextCreate(request, smContext)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	if errRsp != nil {
		return "", errRsp, nil
	}

	responseStatus := producer.SendPFCPRules(smContext)
	if responseStatus != context.SessionEstablishSuccess {
		if smContext != nil {
			go func() {
				err := producer.SendPduSessN1N2Transfer(smContext, false)
				if err != nil {
					logger.SmfLog.Errorf("error transferring N1N2: %v", err)
				}
			}()
		}
		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}
	go func() {
		err := producer.SendPduSessN1N2Transfer(smContext, true)
		if err != nil {
			logger.SmfLog.Errorf("error transferring N1N2: %v", err)
		}
	}()
	return location, nil, nil
}
