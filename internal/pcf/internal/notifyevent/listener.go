package notifyevent

import (
	"github.com/yeastengine/ella/internal/logger"
)

type NotifyListener struct{}

func (l NotifyListener) Listen(event interface{}) {
	switch event := event.(type) {
	case SendSMpolicyUpdateNotifyEvent:
		event.Handle()
	case SendSMpolicyTerminationNotifyEvent:
		event.Handle()
	default:
		logger.PcfLog.Warnf("registered an invalid user event: %T\n", event)
	}
}
