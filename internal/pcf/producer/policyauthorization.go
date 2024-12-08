package producer

import (
	"context"
	"net/http"

	"github.com/omec-project/openapi/models"
	pcf_context "github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/util"
)

func SendAppSessionTermination(appSession *pcf_context.AppSessionData, request models.TerminationInfo) {
	logger.PolicyAuthorizationlog.Debugf("Send App Session Termination")
	if appSession == nil {
		logger.PolicyAuthorizationlog.Warnln("Send App Session Termination Error[appSession is nil]")
		return
	}
	uri := appSession.AppSessionContext.AscReqData.NotifUri
	if uri != "" {
		request.ResUri = util.GetResourceUri(models.ServiceName_NPCF_POLICYAUTHORIZATION, appSession.AppSessionId)
		client := util.GetNpcfPolicyAuthorizationCallbackClient()
		httpResponse, err := client.PolicyAuthorizationTerminateRequestApi.PolicyAuthorizationTerminateRequest(
			context.Background(), uri, request)
		if err != nil {
			if httpResponse != nil {
				logger.PolicyAuthorizationlog.Warnf("Send App Session Termination Error[%s]", httpResponse.Status)
			} else {
				logger.PolicyAuthorizationlog.Warnf("Send App Session Termination Failed[%s]", err.Error())
			}
			return
		} else if httpResponse == nil {
			logger.PolicyAuthorizationlog.Warnln("Send App Session Termination Failed[HTTP Response is nil]")
			return
		}
		defer func() {
			if rspCloseErr := httpResponse.Body.Close(); rspCloseErr != nil {
				logger.PolicyAuthorizationlog.Errorf(
					"PolicyAuthorizationTerminateRequest response body cannot close: %+v", rspCloseErr)
			}
		}()
		if httpResponse.StatusCode != http.StatusOK && httpResponse.StatusCode != http.StatusNoContent {
			logger.PolicyAuthorizationlog.Warnf("Send App Session Termination Failed")
		} else {
			logger.PolicyAuthorizationlog.Debugf("Send App Session Termination Success")
		}
	}
}
