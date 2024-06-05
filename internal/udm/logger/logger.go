package logger

import (
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log         *logrus.Logger
	AppLog      *logrus.Entry
	InitLog     *logrus.Entry
	CfgLog      *logrus.Entry
	Handlelog   *logrus.Entry
	HttpLog     *logrus.Entry
	UeauLog     *logrus.Entry
	UecmLog     *logrus.Entry
	SdmLog      *logrus.Entry
	PpLog       *logrus.Entry
	EeLog       *logrus.Entry
	UtilLog     *logrus.Entry
	CallbackLog *logrus.Entry
	ContextLog  *logrus.Entry
	ConsumerLog *logrus.Entry
	GinLog      *logrus.Entry
	GrpcLog     *logrus.Entry
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

	AppLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "Init"})
	CfgLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "CFG"})
	Handlelog = log.WithFields(logrus.Fields{"component": "UDM", "category": "HDLR"})
	HttpLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "HTTP"})
	UeauLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "UEAU"})
	UecmLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "UECM"})
	SdmLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "SDM"})
	PpLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "PP"})
	EeLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "EE"})
	UtilLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "Util"})
	CallbackLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "CB"})
	ContextLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "CTX"})
	ConsumerLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "Consumer"})
	GinLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "GIN"})
	GrpcLog = log.WithFields(logrus.Fields{"component": "UDM", "category": "GRPC"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
