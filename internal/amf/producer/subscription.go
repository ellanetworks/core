package producer

import (
	"net/http"
	"reflect"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/amf/context"
	"github.com/yeastengine/canard/internal/amf/logger"
)

// TS 29.518 5.2.2.5.1
func HandleAMFStatusChangeSubscribeRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle AMF Status Change Subscribe Request")

	subscriptionDataReq := request.Body.(models.SubscriptionData)

	subscriptionDataRsp, locationHeader, problemDetails := AMFStatusChangeSubscribeProcedure(subscriptionDataReq)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}

	headers := http.Header{
		"Location": {locationHeader},
	}
	return httpwrapper.NewResponse(http.StatusCreated, headers, subscriptionDataRsp)
}

func AMFStatusChangeSubscribeProcedure(subscriptionDataReq models.SubscriptionData) (
	subscriptionDataRsp models.SubscriptionData, locationHeader string, problemDetails *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	for _, guami := range subscriptionDataReq.GuamiList {
		for _, servedGumi := range amfSelf.ServedGuamiList {
			if reflect.DeepEqual(guami, servedGumi) {
				// AMF status is available
				subscriptionDataRsp.GuamiList = append(subscriptionDataRsp.GuamiList, guami)
			}
		}
	}

	if subscriptionDataRsp.GuamiList != nil {
		newSubscriptionID := amfSelf.NewAMFStatusSubscription(subscriptionDataReq)
		locationHeader = subscriptionDataReq.AmfStatusUri + "/" + newSubscriptionID
		logger.CommLog.Infof("new AMF Status Subscription[%s]", newSubscriptionID)
		return
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  "UNSPECIFIED",
		}
		return
	}
}

// TS 29.518 5.2.2.5.2
func HandleAMFStatusChangeUnSubscribeRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle AMF Status Change UnSubscribe Request")

	subscriptionID := request.Params["subscriptionId"]

	problemDetails := AMFStatusChangeUnSubscribeProcedure(subscriptionID)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusNoContent, nil, nil)
	}
}

func AMFStatusChangeUnSubscribeProcedure(subscriptionID string) (problemDetails *models.ProblemDetails) {
	amfSelf := context.AMF_Self()

	if _, ok := amfSelf.FindAMFStatusSubscription(subscriptionID); !ok {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusNotFound,
			Cause:  "SUBSCRIPTION_NOT_FOUND",
		}
	} else {
		logger.CommLog.Debugf("Delete AMF status subscription[%s]", subscriptionID)
		amfSelf.DeleteAMFStatusSubscription(subscriptionID)
	}
	return
}

// TS 29.518 5.2.2.5.1.3
func HandleAMFStatusChangeSubscribeModify(request *httpwrapper.Request) *httpwrapper.Response {
	logger.CommLog.Info("Handle AMF Status Change Subscribe Modify Request")

	updateSubscriptionData := request.Body.(models.SubscriptionData)
	subscriptionID := request.Params["subscriptionId"]

	updatedSubscriptionData, problemDetails := AMFStatusChangeSubscribeModifyProcedure(subscriptionID,
		updateSubscriptionData)
	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusAccepted, nil, updatedSubscriptionData)
	}
}

func AMFStatusChangeSubscribeModifyProcedure(subscriptionID string, subscriptionData models.SubscriptionData) (
	*models.SubscriptionData, *models.ProblemDetails,
) {
	amfSelf := context.AMF_Self()

	if currentSubscriptionData, ok := amfSelf.FindAMFStatusSubscription(subscriptionID); !ok {
		problemDetails := &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  "Forbidden",
		}
		return nil, problemDetails
	} else {
		logger.CommLog.Debugf("Modify AMF status subscription[%s]", subscriptionID)

		currentSubscriptionData.GuamiList = currentSubscriptionData.GuamiList[:0]

		currentSubscriptionData.GuamiList = append(currentSubscriptionData.GuamiList, subscriptionData.GuamiList...)
		currentSubscriptionData.AmfStatusUri = subscriptionData.AmfStatusUri

		amfSelf.AMFStatusSubscriptions.Store(subscriptionID, currentSubscriptionData)
		return currentSubscriptionData, nil
	}
}
