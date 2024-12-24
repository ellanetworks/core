package producer

import (
	"context"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	pcf_context "github.com/ellanetworks/core/internal/pcf/context"
	"github.com/ellanetworks/core/internal/pcf/util"
	"github.com/omec-project/openapi/models"
)

func SendAppSessionTermination(appSession *pcf_context.AppSessionData, request models.TerminationInfo) {
	logger.PcfLog.Debugf("Send App Session Termination")
	if appSession == nil {
		logger.PcfLog.Warnln("Send App Session Termination Error[appSession is nil]")
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
				logger.PcfLog.Warnf("Send App Session Termination Error[%s]", httpResponse.Status)
			} else {
				logger.PcfLog.Warnf("Send App Session Termination Failed[%s]", err.Error())
			}
			return
		} else if httpResponse == nil {
			logger.PcfLog.Warnln("Send App Session Termination Failed[HTTP Response is nil]")
			return
		}
		defer func() {
			if rspCloseErr := httpResponse.Body.Close(); rspCloseErr != nil {
				logger.PcfLog.Errorf(
					"PolicyAuthorizationTerminateRequest response body cannot close: %+v", rspCloseErr)
			}
		}()
		if httpResponse.StatusCode != http.StatusOK && httpResponse.StatusCode != http.StatusNoContent {
			logger.PcfLog.Warnf("Send App Session Termination Failed")
		} else {
			logger.PcfLog.Debugf("Send App Session Termination Success")
		}
	}
}
