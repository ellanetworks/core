package callback

import (
	"context"

	"github.com/omec-project/openapi/Nudr_DataRepository"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
)

func SendOnDataChangeNotify(ueId string, notifyItems []models.NotifyItem) {
	udrSelf := udr_context.UDR_Self()
	configuration := Nudr_DataRepository.NewConfiguration()
	client := Nudr_DataRepository.NewAPIClient(configuration)

	for _, subscriptionDataSubscription := range udrSelf.SubscriptionDataSubscriptions {
		if ueId == subscriptionDataSubscription.UeId {
			onDataChangeNotifyUrl := subscriptionDataSubscription.CallbackReference

			dataChangeNotify := models.DataChangeNotify{}
			dataChangeNotify.UeId = ueId
			dataChangeNotify.OriginalCallbackReference = []string{subscriptionDataSubscription.OriginalCallbackReference}
			dataChangeNotify.NotifyItems = notifyItems
			httpResponse, err := client.DataChangeNotifyCallbackDocumentApi.OnDataChangeNotify(context.TODO(),
				onDataChangeNotifyUrl, dataChangeNotify)
			if err != nil {
				if httpResponse == nil {
					logger.UdrLog.Errorln(err.Error())
				} else if err.Error() != httpResponse.Status {
					logger.UdrLog.Errorln(err.Error())
				}
			}
		}
	}
}
