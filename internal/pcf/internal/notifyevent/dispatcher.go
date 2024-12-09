package notifyevent

import (
	"github.com/omec-project/openapi/models"
	"github.com/tim-ywliu/event"
	"github.com/yeastengine/ella/internal/logger"
)

var notifyDispatcher *event.Dispatcher

func RegisterNotifyDispatcher() error {
	notifyDispatcher = event.NewDispatcher()
	if err := notifyDispatcher.Register(NotifyListener{},
		SendSMpolicyUpdateNotifyEventName,
		SendSMpolicyTerminationNotifyEventName); err != nil {
		return err
	}
	return nil
}

func DispatchSendSMPolicyUpdateNotifyEvent(uri string, request *models.SmPolicyNotification) {
	if notifyDispatcher == nil {
		logger.PcfLog.Errorf("notifyDispatcher is nil")
	}
	err := notifyDispatcher.Dispatch(SendSMpolicyUpdateNotifyEventName, SendSMpolicyUpdateNotifyEvent{
		uri:     uri,
		request: request,
	})
	if err != nil {
		logger.PcfLog.Errorln(err)
	}
}
