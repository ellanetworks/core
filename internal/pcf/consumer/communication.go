package consumer

import (
	"context"
	"fmt"
	"strings"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	pcf_context "github.com/yeastengine/canard/internal/pcf/context"
	"github.com/yeastengine/canard/internal/pcf/logger"
	"github.com/yeastengine/canard/internal/pcf/util"
)

func AmfStatusChangeSubscribe(amfUri string, guamiList []models.Guami) (
	problemDetails *models.ProblemDetails, err error,
) {
	logger.Consumerlog.Debugf("PCF Subscribe to AMF status[%+v]", amfUri)
	pcfSelf := pcf_context.PCF_Self()
	client := util.GetNamfClient(amfUri)

	subscriptionData := models.SubscriptionData{
		AmfStatusUri: fmt.Sprintf("%s/npcf-callback/v1/amfstatus", pcfSelf.GetIPv4Uri()),
		GuamiList:    guamiList,
	}

	res, httpResp, localErr := client.SubscriptionsCollectionDocumentApi.AMFStatusChangeSubscribe(context.Background(), subscriptionData)
	if localErr == nil {
		locationHeader := httpResp.Header.Get("Location")
		logger.Consumerlog.Debugf("location header: %+v", locationHeader)

		subscriptionID := locationHeader[strings.LastIndex(locationHeader, "/")+1:]
		amfStatusSubsData := pcf_context.AMFStatusSubscriptionData{
			AmfUri:       amfUri,
			AmfStatusUri: res.AmfStatusUri,
			GuamiList:    res.GuamiList,
		}
		pcfSelf.NewAmfStatusSubscription(subscriptionID, amfStatusSubsData)
	} else if httpResp != nil {
		if httpResp.Status != localErr.Error() {
			err = localErr
			return nil, err
		}
		problem := localErr.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails)
		problemDetails = &problem
	} else {
		err = openapi.ReportError("%s: server no response", amfUri)
	}
	return problemDetails, err
}
