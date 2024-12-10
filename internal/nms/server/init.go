package server

import (
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/logger"
)

func Start(port int, cert_file string, key_file string) {
	router := logger_util.NewGinWithZap(logger.NmsLog)
	AddUiService(router)
	AddApiService(router)

	router.Use(cors.New(cors.Config{
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
		httpAddr := ":" + strconv.Itoa(port)
		logger.NmsLog.Infoln("NMS HTTP addr:", httpAddr, port)
		server, err := http2_util.NewServer(httpAddr, "", router)
		if server == nil {
			logger.NmsLog.Errorln("NMS server is nil")
			return
		}
		if err != nil {
			logger.NmsLog.Errorln("couldn't create NMS server:", err)
			return
		}
		err = server.ListenAndServeTLS(cert_file, key_file)
		if err != nil {
			logger.NmsLog.Errorln("couldn't start NMS server:", err)
		}
	}()

	select {}
}
