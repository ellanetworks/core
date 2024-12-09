package server

import (
	_ "net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/gin-contrib/cors"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/config"
)

type NMS struct{}

func (nms *NMS) Initialize(c config.Configuration) {
	config.InitConfigFactory(c)
}

func (nms *NMS) Start() {
	subconfig_router := logger_util.NewGinWithZap(logger.NMSLog)
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
		logger.NMSLog.Infoln("NMS HTTP addr:", httpAddr, config.Config.CfgPort)
		logger.NMSLog.Infoln(subconfig_router.Run(httpAddr))
		logger.NMSLog.Infoln("NMS stopped/terminated/not-started ")
	}()

	select {}
}
