package consumer

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
)

func SendRemoveSubscriptionProcedure(notificationData models.NotificationData) {
	logger.ConsumerLog.Infof("[SMF] Send Remove Subscription Procedure")
	nfInstanceId := notificationData.NfInstanceUri[strings.LastIndex(notificationData.NfInstanceUri, "/")+1:]

	if subscriptionId, ok := smf_context.SMF_Self().NfStatusSubscriptions.Load(nfInstanceId); ok {
		logger.ConsumerLog.Debugf("SubscriptionId of nfInstance %v is %v", nfInstanceId, subscriptionId.(string))
		problemDetails, err := SendRemoveSubscription(subscriptionId.(string))
		if problemDetails != nil {
			logger.ConsumerLog.Errorf("Remove NF Subscription Failed Problem[%+v]", problemDetails)
		} else if err != nil {
			logger.ConsumerLog.Errorf("Remove NF Subscription Error[%+v]", err)
		} else {
			logger.ConsumerLog.Infoln("[SMF] Remove NF Subscription successful")
			smf_context.SMF_Self().NfStatusSubscriptions.Delete(nfInstanceId)
		}
	} else {
		logger.ConsumerLog.Infof("nfinstance %v not found in map", nfInstanceId)
	}
}

func SendRemoveSubscription(subscriptionId string) (problemDetails *models.ProblemDetails, err error) {
	logger.ConsumerLog.Infof("[SMF] Send Remove Subscription for Subscription Id: %v", subscriptionId)

	var res *http.Response
	res, err = smf_context.SMF_Self().NFManagementClient.SubscriptionIDDocumentApi.RemoveSubscription(context.Background(), subscriptionId)
	if err == nil {
		return
	} else if res != nil {
		defer func() {
			if bodyCloseErr := res.Body.Close(); bodyCloseErr != nil {
				err = fmt.Errorf("RemoveSubscription' response body cannot close: %+w", bodyCloseErr)
			}
		}()
		if res.Status != err.Error() {
			return
		}
		problem := err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("server no response")
	}
	return
}
