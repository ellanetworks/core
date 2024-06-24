package webui_service

import (
	_ "net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/webui/backend/factory"
	"github.com/yeastengine/ella/internal/webui/backend/logger"
	"github.com/yeastengine/ella/internal/webui/backend/webui_context"
	"github.com/yeastengine/ella/internal/webui/configapi"
	"github.com/yeastengine/ella/internal/webui/configmodels"
	"github.com/yeastengine/ella/internal/webui/dbadapter"
	gServ "github.com/yeastengine/ella/internal/webui/proto/server"
)

type WEBUI struct{}

type Config struct {
	webuicfg string
}

var config Config

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (webui *WEBUI) Initialize(c *cli.Context) {
	config = Config{
		webuicfg: c.String("webuicfg"),
	}

	if config.webuicfg != "" {
		if err := factory.InitConfigFactory(config.webuicfg); err != nil {
			panic(err)
		}
	} else {
		DefaultWebUIConfigPath := path_util.Free5gcPath("free5gc/config/webuicfg.yaml")
		if err := factory.InitConfigFactory(DefaultWebUIConfigPath); err != nil {
			panic(err)
		}
	}

	webui.setLogLevel()
}

func (webui *WEBUI) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.WebUIConfig.Logger.WEBUI.DebugLevel); err != nil {
		initLog.Warnf("WebUI Log level [%s] is invalid, set to [info] level",
			factory.WebUIConfig.Logger.WEBUI.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("WebUI Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.WebUIConfig.Logger.WEBUI.ReportCaller)
}

func (webui *WEBUI) Start() {
	// get config file info from WebUIConfig
	mongodb := factory.WebUIConfig.Configuration.Mongodb

	// Connect to MongoDB
	dbadapter.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)

	initLog.Infoln("WebUI Server started")

	/* First HTTP Server running at port to receive Config from ROC */
	subconfig_router := logger_util.NewGinWithLogrus(logger.GinLog)
	configapi.AddServiceSub(subconfig_router)
	configapi.AddService(subconfig_router)

	configMsgChan := make(chan *configmodels.ConfigMessage, 10)
	configapi.SetChannel(configMsgChan)

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
		httpAddr := ":" + strconv.Itoa(factory.WebUIConfig.Configuration.CfgPort)
		initLog.Infoln("Webui HTTP addr:", httpAddr, factory.WebUIConfig.Configuration.CfgPort)
		if factory.WebUIConfig.Info.HttpVersion == 2 {
			server, err := http2_util.NewServer(httpAddr, "", subconfig_router)
			if server == nil {
				initLog.Error("Initialize HTTP-2 server failed:", err)
				return
			}

			if err != nil {
				initLog.Warnln("Initialize HTTP-2 server:", err)
				return
			}

			err = server.ListenAndServe()
			if err != nil {
				initLog.Fatalln("HTTP server setup failed:", err)
				return
			}
		} else {
			initLog.Infoln(subconfig_router.Run(httpAddr))
			initLog.Infoln("Webserver stopped/terminated/not-started ")
		}
	}()
	/* First HTTP server end */

	self := webui_context.WEBUI_Self()
	self.UpdateNfProfiles()

	var host string = "0.0.0.0:9876"
	confServ := &gServ.ConfigServer{}
	go gServ.StartServer(host, confServ, configMsgChan)

	select {}
}
