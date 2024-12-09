package notifyevent

import (
	"context"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/tim-ywliu/event"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/pcf/util"
)

const SendSMpolicyTerminationNotifyEventName event.Name = "SendSMpolicyTerminationNotify"

type SendSMpolicyTerminationNotifyEvent struct {
	request *models.TerminationNotification
	uri     string
}

func (e SendSMpolicyTerminationNotifyEvent) Handle() {
	logger.PcfLog.Infof("Handle SendSMpolicyTerminationNotifyEvent\n")
	if e.uri == "" {
		logger.PcfLog.Warnln("SM Policy Termination Request Notification Error[URI is empty]")
		return
	}
	client := util.GetNpcfSMPolicyCallbackClient()
	logger.PcfLog.Infof("SM Policy Termination Request Notification to SMF")
	rsp, err := client.DefaultCallbackApi.SmPolicyControlTerminationRequestNotification(context.Background(), e.uri, *e.request)
	if err != nil {
		if rsp != nil {
			logger.PcfLog.Warnf("SM Policy Termination Request Notification Error[%s]", rsp.Status)
		} else {
			logger.PcfLog.Warnf("SM Policy Termination Request Notification Error[%s]", err.Error())
		}
		return
	} else if rsp == nil {
		logger.PcfLog.Warnln("SM Policy Termination Request Notification Error[HTTP Response is nil]")
		return
	}
	defer func() {
		if resCloseErr := rsp.Body.Close(); resCloseErr != nil {
			logger.PcfLog.Errorf("NFInstancesStoreApi response body cannot close: %+v", resCloseErr)
		}
	}()
	if rsp.StatusCode != http.StatusNoContent {
		logger.PcfLog.Warnf("SM Policy Termination Request Notification  Failed")
	} else {
		logger.PcfLog.Debugf("SM Policy Termination Request Notification Success")
	}
}
