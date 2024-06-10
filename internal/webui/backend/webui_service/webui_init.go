package webui_service

import (
	"fmt"
	"net/http"
	_ "net/http"
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	logger_util "github.com/omec-project/util/logger"
	mongoDBLibLogger "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	pathUtilLogger "github.com/omec-project/util/path_util/logger"
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

type (
	// Config information.
	Config struct {
		webuicfg string
	}
)

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
	if factory.WebUIConfig.Logger == nil {
		initLog.Warnln("Webconsole config without log level setting!!!")
		return
	}

	if factory.WebUIConfig.Logger.WEBUI != nil {
		if factory.WebUIConfig.Logger.WEBUI.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.WebUIConfig.Logger.WEBUI.DebugLevel); err != nil {
				initLog.Warnf("WebUI Log level [%s] is invalid, set to [info] level",
					factory.WebUIConfig.Logger.WEBUI.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				initLog.Infof("WebUI Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Warnln("WebUI Log level not set. Default set to [info] level")
			logger.SetLogLevel(logrus.InfoLevel)
		}
		logger.SetReportCaller(factory.WebUIConfig.Logger.WEBUI.ReportCaller)
	}

	if factory.WebUIConfig.Logger.PathUtil != nil {
		if factory.WebUIConfig.Logger.PathUtil.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.WebUIConfig.Logger.PathUtil.DebugLevel); err != nil {
				pathUtilLogger.PathLog.Warnf("PathUtil Log level [%s] is invalid, set to [info] level",
					factory.WebUIConfig.Logger.PathUtil.DebugLevel)
				pathUtilLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pathUtilLogger.SetLogLevel(level)
			}
		} else {
			pathUtilLogger.PathLog.Warnln("PathUtil Log level not set. Default set to [info] level")
			pathUtilLogger.SetLogLevel(logrus.InfoLevel)
		}
		pathUtilLogger.SetReportCaller(factory.WebUIConfig.Logger.PathUtil.ReportCaller)
	}

	if factory.WebUIConfig.Logger.MongoDBLibrary != nil {
		if factory.WebUIConfig.Logger.MongoDBLibrary.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.WebUIConfig.Logger.MongoDBLibrary.DebugLevel); err != nil {
				mongoDBLibLogger.AppLog.Warnf("MongoDBLibrary Log level [%s] is invalid, set to [info] level",
					factory.WebUIConfig.Logger.MongoDBLibrary.DebugLevel)
				mongoDBLibLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				mongoDBLibLogger.SetLogLevel(level)
			}
		} else {
			mongoDBLibLogger.AppLog.Warnln("MongoDBLibrary Log level not set. Default set to [info] level")
			mongoDBLibLogger.SetLogLevel(logrus.InfoLevel)
		}
		mongoDBLibLogger.SetReportCaller(factory.WebUIConfig.Logger.MongoDBLibrary.ReportCaller)
	}
}

func (webui *WEBUI) Start() {
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
		initLog.Infoln(subconfig_router.Run(httpAddr))
		initLog.Infoln("Webserver stopped/terminated/not-started ")
	}()

	self := webui_context.WEBUI_Self()
	self.UpdateNfProfiles()

	var host string = "0.0.0.0:9876"
	confServ := &gServ.ConfigServer{}
	go gServ.StartServer(host, confServ, configMsgChan)

	// fetch one time configuration from the simapp/roc on startup
	// this is to fetch existing config
	go fetchConfigAdapater()

	select {}
}

func fetchConfigAdapater() {
	for {
		if (factory.WebUIConfig.Configuration == nil) ||
			(factory.WebUIConfig.Configuration.RocEnd == nil) ||
			(!factory.WebUIConfig.Configuration.RocEnd.Enabled) ||
			(factory.WebUIConfig.Configuration.RocEnd.SyncUrl == "") {
			time.Sleep(1 * time.Second)
			// fmt.Printf("Continue polling config change %v ", factory.WebUIConfig.Configuration)
			continue
		}

		client := &http.Client{}
		httpend := factory.WebUIConfig.Configuration.RocEnd.SyncUrl
		req, err := http.NewRequest(http.MethodPost, httpend, nil)
		// Handle Error
		if err != nil {
			fmt.Printf("An Error Occurred %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		// set the request header Content-Type for json
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("An Error Occurred %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		err = resp.Body.Close()
		if err != nil {
			fmt.Printf("An Error Occurred %v\n", err)
		}
		fmt.Printf("Fetching config from simapp/roc. Response code = %d \n", resp.StatusCode)
		break
	}
}
