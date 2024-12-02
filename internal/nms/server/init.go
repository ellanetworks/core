package server

import (
	_ "net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/gin-contrib/cors"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/nms/config"
	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/logger"
)

type NMS struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (nms *NMS) Initialize(c config.Configuration) {
	config.InitConfigFactory(c)
	nms.setLogLevel()
}

func (nms *NMS) setLogLevel() {
	if level, err := logrus.ParseLevel(config.Config.Logger.WEBUI.DebugLevel); err != nil {
		initLog.Warnf("NMS Log level [%s] is invalid, set to [info] level",
			config.Config.Logger.WEBUI.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("NMS Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(config.Config.Logger.WEBUI.ReportCaller)
}

func (nms *NMS) Start() {
	mongodb := config.Config.Mongodb

	db.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)

	subconfig_router := logger_util.NewGinWithLogrus(logger.GinLog)
	AddUiService(subconfig_router)
	AddServiceSub(subconfig_router)
	AddService(subconfig_router)

	subconfig_router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{
			"Origin", "Content-Length", "Content-Type", "User-Agent",
			"Referrer", "Host", "Token", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           86400,
	}))

	go func() {
		httpAddr := ":" + strconv.Itoa(config.Config.CfgPort)
		initLog.Infoln("NMS HTTP addr:", httpAddr, config.Config.CfgPort)
		initLog.Infoln(subconfig_router.Run(httpAddr))
		initLog.Infoln("NMS stopped/terminated/not-started ")
	}()

	select {}
}
