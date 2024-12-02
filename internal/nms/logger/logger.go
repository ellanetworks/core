package logger

import (
	"os"
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/logger_conf"
	"github.com/sirupsen/logrus"
)

var (
	log        *logrus.Logger
	AppLog     *logrus.Entry
	InitLog    *logrus.Entry
	NMSLog     *logrus.Entry
	ContextLog *logrus.Entry
	GinLog     *logrus.Entry
	GrpcLog    *logrus.Entry
	ConfigLog  *logrus.Entry
	DbLog      *logrus.Entry
)

func init() {
	log = logrus.New()
	log.SetReportCaller(false)

	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category"},
	}

	free5gcLogHook, err := logger_util.NewFileHook(logger_conf.Free5gcLogFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o666)
	if err == nil {
		log.Hooks.Add(free5gcLogHook)
	}

	selfLogHook, err := logger_util.NewFileHook(
		logger_conf.Free5gcLogDir+"webconsole.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o666)
	if err == nil {
		log.Hooks.Add(selfLogHook)
	}

	AppLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "Init"})
	NMSLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "NMS"})
	ContextLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "Context"})
	GinLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "GIN"})
	GrpcLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "GRPC"})
	ConfigLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "CONFIG"})
	DbLog = log.WithFields(logrus.Fields{"component": "NMS", "category": "DB"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
