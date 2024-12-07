package pdusession

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/smf/context"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/fsm"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/msgtypes/svcmsgtypes"
	"github.com/yeastengine/ella/internal/smf/transaction"
)

// HTTPPostSmContexts - Create SM Context
func HTTPPostSmContexts(c *gin.Context) {
	logger.PduSessLog.Info("Receive Create SM Context Request")
	var request models.PostSmContextsRequest

	request.JsonData = new(models.SmContextCreateData)

	s := strings.Split(c.GetHeader("Content-Type"), ";")
	var err error
	switch s[0] {
	case "application/json":
		err = c.ShouldBindJSON(request.JsonData)
	case "multipart/related":
		err = c.ShouldBindWith(&request, openapi.MultipartRelatedBinding{})
	}

	if err != nil {
		problemDetail := "[Request Body] " + err.Error()
		rsp := models.ProblemDetails{
			Title:  "Malformed request syntax",
			Status: http.StatusBadRequest,
			Detail: problemDetail,
		}
		logger.PduSessLog.Errorln(problemDetail)
		c.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := httpwrapper.NewRequest(c.Request, request)
	txn := transaction.NewTransaction(req.Body.(models.PostSmContextsRequest), nil, svcmsgtypes.SmfMsgType(svcmsgtypes.CreateSmContext))

	go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
	<-txn.Status // wait for txn to complete at SMF
	HTTPResponse := txn.Rsp.(*httpwrapper.Response)
	smContext := txn.Ctxt.(*smf_context.SMContext)

	// Http Response to AMF

	for key, val := range HTTPResponse.Header {
		c.Header(key, val[0])
	}
	switch HTTPResponse.Status {
	case http.StatusCreated,
		http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		c.Render(HTTPResponse.Status, openapi.MultipartRelatedRender{Data: HTTPResponse.Body})
	default:
		c.JSON(HTTPResponse.Status, HTTPResponse.Body)
	}

	go func(smContext *smf_context.SMContext) {
		var txn *transaction.Transaction
		if HTTPResponse.Status == http.StatusCreated {
			txn = transaction.NewTransaction(nil, nil, svcmsgtypes.SmfMsgType(svcmsgtypes.PfcpSessCreate))
			txn.Ctxt = smContext
			go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
			<-txn.Status
		} else {
			smf_context.RemoveSMContext(smContext.Ref)
		}
	}(smContext)
}

// CreateSmContext - Creates SM Context
func CreateSmContext(request models.PostSmContextsRequest) (*models.PostSmContextsResponse, *models.PostSmContextsErrorResponse, error) {
	logger.PduSessLog.Info("Processing Create SM Context Request")

	// Ensure request data is present
	if request.JsonData == nil {
		errResponse := &models.PostSmContextsErrorResponse{
			JsonData: &models.SmContextCreateError{},
		}
		return nil, errResponse, errors.New("missing JsonData in request")
	}

	// Create transaction
	txn := transaction.NewTransaction(request, nil, svcmsgtypes.SmfMsgType(svcmsgtypes.CreateSmContext))

	// Start FSM lifecycle
	go txn.StartTxnLifeCycle(fsm.SmfTxnFsmHandle)
	<-txn.Status // Wait for transaction to complete

	// Handle transaction response
	HTTPResponse, ok := txn.Rsp.(*httpwrapper.Response)
	if !ok {
		return nil, nil, errors.New("unexpected transaction response type")
	}

	// Check for SM Context in transaction context
	_, ok = txn.Ctxt.(*context.SMContext)
	if !ok && HTTPResponse.Status == http.StatusCreated {
		return nil, nil, errors.New("failed to retrieve SMContext from transaction context")
	}

	// Process response based on HTTP status
	switch HTTPResponse.Status {
	case http.StatusCreated:
		// Successful creation
		// Print content of the response body
		fmt.Println("HTTPResponse.Body: ", HTTPResponse.Body)
		response, ok := HTTPResponse.Body.(models.PostSmContextsResponse)
		if !ok {
			return nil, nil, errors.New("unexpected response body type for successful creation")
		}
		return &response, nil, nil

	case http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		// Handle errors
		errResponse, ok := HTTPResponse.Body.(*models.PostSmContextsErrorResponse)
		if !ok {
			return nil, nil, errors.New("unexpected response body type for error")
		}
		return nil, errResponse, nil

	default:
		// Unexpected status
		return nil, nil, errors.New("unexpected HTTP status code")
	}
}
