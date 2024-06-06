package logger

import (
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log     *logrus.Logger
	AppLog  *logrus.Entry
	PfcpLog *logrus.Entry
	CfgLog  *logrus.Entry
)

func init() {
	log = logrus.New()
	log.SetReportCaller(true)

	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category"},
	}

	AppLog = log.WithFields(logrus.Fields{"component": "UADP", "category": "App"})
	PfcpLog = log.WithFields(logrus.Fields{"component": "UADP", "category": "Pfcp"})
	CfgLog = log.WithFields(logrus.Fields{"component": "UADP", "category": "Config"})

	log.SetLevel(logrus.DebugLevel)
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
