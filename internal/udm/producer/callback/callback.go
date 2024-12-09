package callback

import (
	"context"
	"net/http"

	"github.com/omec-project/openapi/Nudm_UEContextManagement"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/logger"
)

func SendOnDeregistrationNotification(ueId string, onDeregistrationNotificationUrl string,
	deregistData models.DeregistrationData,
) *models.ProblemDetails {
	configuration := Nudm_UEContextManagement.NewConfiguration()
	clientAPI := Nudm_UEContextManagement.NewAPIClient(configuration)

	httpResponse, err := clientAPI.DeregistrationNotificationCallbackApi.DeregistrationNotify(
		context.TODO(), onDeregistrationNotificationUrl, deregistData)
	if err != nil {
		if httpResponse == nil {
			logger.UdmLog.Error(err.Error())
			problemDetails := &models.ProblemDetails{
				Status: http.StatusInternalServerError,
				Cause:  "DEREGISTRATION_NOTIFICATION_ERROR",
				Detail: err.Error(),
			}

			return problemDetails
		} else {
			logger.UdmLog.Errorln(err.Error())
			problemDetails := &models.ProblemDetails{
				Status: int32(httpResponse.StatusCode),
				Cause:  "DEREGISTRATION_NOTIFICATION_ERROR",
				Detail: err.Error(),
			}

			return problemDetails
		}
	}
	defer func() {
		if rspCloseErr := httpResponse.Body.Close(); rspCloseErr != nil {
			logger.UdmLog.Errorf("DeregistrationNotify response body cannot close: %+v", rspCloseErr)
		}
	}()

	return nil
}
