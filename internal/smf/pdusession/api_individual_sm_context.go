// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	coreModels "github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
	"github.com/ellanetworks/core/internal/smf/util"
	"github.com/omec-project/openapi/models"
)

func HandleStateActiveEventPduSessRelease(request coreModels.ReleaseSmContextRequest, smCtxt *context.SMContext) (context.SMContextState, *util.Response, error) {
	rsp, err := producer.HandlePDUSessionSMContextRelease(request, smCtxt)
	if err != nil {
		return context.SmStateInit, rsp, err
	}

	return context.SmStateInit, rsp, nil
}

func ReleaseSmContext(smContextRef string, releaseSmContextRequest coreModels.ReleaseSmContextRequest) error {
	logger.SmfLog.Info("Processing Release SM Context Request")

	// Validate the request content
	if releaseSmContextRequest.JsonData == nil {
		return errors.New("release request is missing JsonData")
	}

	// Start transaction
	ctxt := context.GetSMContext(smContextRef)
	nextState, rsp, err := HandleStateActiveEventPduSessRelease(releaseSmContextRequest, ctxt)
	ctxt.ChangeState(nextState)
	if err != nil {
		logger.SmfLog.Errorf("error processing state machine transaction")
		if ctxt == nil {
			logger.SmfLog.Warnf("PDUSessionSMContextRelease [%s] is not found", smContextRef)
			// 4xx/5xx Error not defined in spec 29502 for Release SM ctxt error
			// Send Not Found
			httpResponse := &util.Response{
				Header: nil,
				Status: http.StatusNotFound,

				Body: &models.ProblemDetails{
					Type:   "Resource Not Found",
					Title:  "SMContext Ref is not found",
					Status: http.StatusNotFound,
				},
			}
			rsp = httpResponse
		}
	}

	// Process response based on HTTP status
	switch rsp.Status {
	case http.StatusNoContent:
		// Successful release
		return nil
	default:
		// Handle errors
		errResponse, ok := rsp.Body.(*models.ProblemDetails)
		if ok {
			logger.SmfLog.Errorf("SM Context release failed: %s", errResponse.Detail)
			return errors.New(errResponse.Detail)
		}
		return errors.New("unexpected error during SM Context release")
	}
}

func HandlePduSessModify(request coreModels.UpdateSmContextRequest, smCtxt *context.SMContext) (context.SMContextState, *util.Response, error) {
	rsp, err := producer.HandlePDUSessionSMContextUpdate(request, smCtxt)
	if err != nil {
		rsp = &util.Response{
			Header: nil,
			Status: http.StatusNotFound,
			Body: models.UpdateSmContextErrorResponse{
				JsonData: &models.SmContextUpdateError{
					UpCnxState: models.UpCnxState_DEACTIVATED,
					Error: &models.ProblemDetails{
						Type:   "Resource Not Found",
						Title:  "SMContext Ref is not found",
						Status: http.StatusNotFound,
					},
				},
			},
		}
		return context.SmStateActive, rsp, fmt.Errorf("error updating pdu session: %v ", err.Error())
	}
	return context.SmStateActive, rsp, nil
}

func UpdateSmContext(smContextRef string, updateSmContextRequest coreModels.UpdateSmContextRequest) (*models.UpdateSmContextResponse, error) {
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

	switch rsp.Status {
	case http.StatusOK, http.StatusNoContent:
		response, ok := rsp.Body.(models.UpdateSmContextResponse)
		if !ok {
			return nil, errors.New("unexpected response body type for successful update")
		}
		return &response, nil

	default:
		errResponse, ok := rsp.Body.(*models.ProblemDetails)
		if ok {
			logger.SmfLog.Errorf("SM Context update failed: %s", errResponse.Detail)
			return nil, errors.New(errResponse.Detail)
		}
		return nil, errors.New("unexpected error during SM Context update")
	}
}
