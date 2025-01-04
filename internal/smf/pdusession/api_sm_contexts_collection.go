// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/fsm"
	"github.com/ellanetworks/core/internal/smf/msgtypes/svcmsgtypes"
	"github.com/ellanetworks/core/internal/smf/transaction"
	"github.com/ellanetworks/core/internal/util/httpwrapper"
	"github.com/omec-project/openapi/models"
)

func CreateSmContext(request models.PostSmContextsRequest) (*models.PostSmContextsResponse, string, *models.PostSmContextsErrorResponse, error) {
	logger.SmfLog.Info("Processing Create SM Context Request")

	// Ensure request data is present
	if request.JsonData == nil {
		errResponse := &models.PostSmContextsErrorResponse{
			JsonData: &models.SmContextCreateError{},
		}
		return nil, "", errResponse, fmt.Errorf("missing JsonData in request")
	}

	// Create transaction
	txn := transaction.NewTransaction(request, nil, svcmsgtypes.CreateSmContext)

	// Start FSM lifecycle
	go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
	<-txn.Status // Wait for transaction to complete

	// Handle transaction response
	HTTPResponse, ok := txn.Rsp.(*httpwrapper.Response)
	if !ok {
		return nil, "", nil, fmt.Errorf("unexpected transaction response type")
	}

	// Check for SM Context in transaction context
	smContext, ok := txn.Ctxt.(*context.SMContext)
	if !ok && HTTPResponse.Status == http.StatusCreated {
		return nil, "", nil, fmt.Errorf("failed to retrieve SMContext from transaction context")
	}

	// Process response based on HTTP status
	switch HTTPResponse.Status {
	case http.StatusCreated:
		// Successful creation
		response, ok := HTTPResponse.Body.(models.PostSmContextsResponse)
		if !ok {
			return nil, "", nil, fmt.Errorf("unexpected response body type for successful creation")
		}

		// Start PfcpSessCreate transaction
		pfcpTxn := transaction.NewTransaction(nil, nil, svcmsgtypes.PfcpSessCreate)
		pfcpTxn.Ctxt = smContext
		go pfcpTxn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
		<-pfcpTxn.Status // Wait for the PFCP session transaction to complete
		smContextRef := HTTPResponse.Header.Get("Location")

		return &response, smContextRef, nil, nil

	case http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		// Handle errors
		errResponse, ok := HTTPResponse.Body.(*models.PostSmContextsErrorResponse)
		if !ok {
			return nil, "", nil, fmt.Errorf("unexpected response body type for error")
		}
		return nil, "", errResponse, nil

	default:
		// Unexpected status
		return nil, "", nil, fmt.Errorf("unexpected HTTP status code")
	}
}
