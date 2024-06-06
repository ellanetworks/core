package logger

import (
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log               *logrus.Logger
	AppLog            *logrus.Entry
	CfgLog            *logrus.Entry
	ContextLog        *logrus.Entry
	FactoryLog        *logrus.Entry
	HandlerLog        *logrus.Entry
	InitLog           *logrus.Entry
	Nsselection       *logrus.Entry
	Nssaiavailability *logrus.Entry
	Util              *logrus.Entry
	ConsumerLog       *logrus.Entry
	GinLog            *logrus.Entry
	GrpcLog           *logrus.Entry
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

	AppLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "App"})
	ContextLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "CTX"})
	FactoryLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "Factory"})
	HandlerLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "HDLR"})
	InitLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "Init"})
	CfgLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "CFG"})
	Nsselection = log.WithFields(logrus.Fields{"component": "NSSF", "category": "NsSelect"})
	Nssaiavailability = log.WithFields(logrus.Fields{"component": "NSSF", "category": "NssaiAvail"})
	Util = log.WithFields(logrus.Fields{"component": "NSSF", "category": "Util"})
	ConsumerLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "Consumer"})
	GinLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "GIN"})
	GrpcLog = log.WithFields(logrus.Fields{"component": "NSSF", "category": "GRPC"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
