package pdusession

import (
	"errors"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/smf/fsm"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/msgtypes/svcmsgtypes"
	"github.com/yeastengine/ella/internal/smf/transaction"
)

func ReleaseSmContext(smContextRef string, releaseSmContextRequest models.ReleaseSmContextRequest) error {
	logger.PduSessLog.Info("Processing Release SM Context Request")

	// Validate the request content
	if releaseSmContextRequest.JsonData == nil {
		return errors.New("release request is missing JsonData")
	}

	// Start transaction
	txn := transaction.NewTransaction(releaseSmContextRequest, nil, svcmsgtypes.SmfMsgType(svcmsgtypes.ReleaseSmContext))
	txn.CtxtKey = smContextRef

	// Execute FSM lifecycle
	go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
	<-txn.Status // Wait for transaction to complete

	// Handle transaction response
	HTTPResponse, ok := txn.Rsp.(*httpwrapper.Response)
	if !ok {
		return errors.New("unexpected transaction response type")
	}

	// Process response based on HTTP status
	switch HTTPResponse.Status {
	case http.StatusNoContent:
		// Successful release
		return nil
	default:
		// Handle errors
		errResponse, ok := HTTPResponse.Body.(*models.ProblemDetails)
		if ok {
			logger.PduSessLog.Errorf("SM Context release failed: %s", errResponse.Detail)
			return errors.New(errResponse.Detail)
		}
		return errors.New("unexpected error during SM Context release")
	}
}

func UpdateSmContext(smContextRef string, updateSmContextRequest models.UpdateSmContextRequest) (*models.UpdateSmContextResponse, error) {
	logger.PduSessLog.Info("Processing Update SM Context Request")

	if smContextRef == "" {
		return nil, errors.New("SM Context reference is missing")
	}

	if updateSmContextRequest.JsonData == nil {
		return nil, errors.New("update request is missing JsonData")
	}

	txn := transaction.NewTransaction(updateSmContextRequest, nil, svcmsgtypes.SmfMsgType(svcmsgtypes.UpdateSmContext))
	txn.CtxtKey = smContextRef

	go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
	<-txn.Status // Wait for transaction to complete

	HTTPResponse, ok := txn.Rsp.(*httpwrapper.Response)
	if !ok {
		return nil, errors.New("unexpected transaction response type")
	}

	switch HTTPResponse.Status {
	case http.StatusOK, http.StatusNoContent:
		response, ok := HTTPResponse.Body.(models.UpdateSmContextResponse)
		if !ok {
			return nil, errors.New("unexpected response body type for successful update")
		}
		return &response, nil

	default:
		errResponse, ok := HTTPResponse.Body.(*models.ProblemDetails)
		if ok {
			logger.PduSessLog.Errorf("SM Context update failed: %s", errResponse.Detail)
			return nil, errors.New(errResponse.Detail)
		}
		return nil, errors.New("unexpected error during SM Context update")
	}
}
