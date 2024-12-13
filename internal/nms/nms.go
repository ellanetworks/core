package nms

import (
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/util/http2_util"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/server"

	logger_util "github.com/omec-project/util/logger"
)

func Start(port int, cert_file string, key_file string) error {
	router := logger_util.NewGinWithZap(logger.NmsLog)
	server.AddUiService(router)
	server.AddApiService(router)

	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
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
	return nil
}
