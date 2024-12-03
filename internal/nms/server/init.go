package server

import (
	_ "net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/gin-contrib/cors"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nms/config"
	"github.com/yeastengine/ella/internal/nms/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type NMS struct{}

func (nms *NMS) Initialize(c config.Configuration) {
	config.InitConfigFactory(c)
	setLogLevel()
}

func setLogLevel() {
	if level, err := zapcore.ParseLevel(config.Config.Logger.WEBUI.DebugLevel); err != nil {
		logger.InitLog.Warnf("NMS Log level [%s] is invalid, set to [info] level",
			config.Config.Logger.WEBUI.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("NMS Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (nms *NMS) Start() {
	subconfig_router := logger_util.NewGinWithZap(logger.GinLog)
	AddUiService(subconfig_router)
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
		logger.InitLog.Infoln("NMS HTTP addr:", httpAddr, config.Config.CfgPort)
		logger.InitLog.Infoln(subconfig_router.Run(httpAddr))
		logger.InitLog.Infoln("NMS stopped/terminated/not-started ")
	}()

	select {}
}
