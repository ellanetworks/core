// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package notifyevent

import (
	"context"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/tim-ywliu/event"
	"github.com/yeastengine/canard/internal/pcf/logger"
	"github.com/yeastengine/canard/internal/pcf/util"
)

const SendSMpolicyUpdateNotifyEventName event.Name = "SendSMpolicyUpdateNotify"

type SendSMpolicyUpdateNotifyEvent struct {
	request *models.SmPolicyNotification
	uri     string
}

func (e SendSMpolicyUpdateNotifyEvent) Handle() {
	logger.NotifyEventLog.Infof("Handle SendSMpolicyUpdateNotifyEvent\n")
	if e.uri == "" {
		logger.NotifyEventLog.Warnln("SM Policy Update Notification Error[URI is empty]")
		return
	}
	client := util.GetNpcfSMPolicyCallbackClient()
	logger.NotifyEventLog.Infof("Send SM Policy Update Notification to SMF")
	_, httpResponse, err := client.DefaultCallbackApi.SmPolicyUpdateNotification(context.Background(), e.uri, *e.request)
	if err != nil {
		if httpResponse != nil {
			logger.NotifyEventLog.Warnf("SM Policy Update Notification Error[%s]", httpResponse.Status)
		} else {
			logger.NotifyEventLog.Warnf("SM Policy Update Notification Failed[%s]", err.Error())
		}
		return
	} else if httpResponse == nil {
		logger.NotifyEventLog.Warnln("SM Policy Update Notification Failed[HTTP Response is nil]")
		return
	}
	defer func() {
		if resCloseErr := httpResponse.Body.Close(); resCloseErr != nil {
			logger.NotifyEventLog.Errorf("NFInstancesStoreApi response body cannot close: %+v", resCloseErr)
		}
	}()
	if httpResponse.StatusCode != http.StatusOK && httpResponse.StatusCode != http.StatusNoContent {
		logger.NotifyEventLog.Warnf("SM Policy Update Notification Failed")
	} else {
		logger.NotifyEventLog.Tracef("SM Policy Update Notification Success")
	}
}
