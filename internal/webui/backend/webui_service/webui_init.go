package webui_service

import (
	"bufio"
	"fmt"
	"net/http"
	_ "net/http"
	_ "net/http/pprof"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/util/http2_util"
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

var webuiCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "webuicfg",
		Usage: "config file",
	},
}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (*WEBUI) GetCliCmd() (flags []cli.Flag) {
	return webuiCLi
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

func (webui *WEBUI) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range webui.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (webui *WEBUI) Start() {
	if factory.WebUIConfig.Configuration.Mode5G {
		// get config file info from WebUIConfig
		mongodb := factory.WebUIConfig.Configuration.Mongodb

		// Connect to MongoDB
		dbadapter.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	}

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

	if factory.WebUIConfig.Configuration.Mode5G {
		self := webui_context.WEBUI_Self()
		self.UpdateNfProfiles()
	}

	// Start grpc Server. This has embedded functionality of sending
	// 4G config over REST Api as well.
	var host string = "0.0.0.0:9876"
	confServ := &gServ.ConfigServer{}
	go gServ.StartServer(host, confServ, configMsgChan)

	// fetch one time configuration from the simapp/roc on startup
	// this is to fetch existing config
	go fetchConfigAdapater()

	// http.ListenAndServe("0.0.0.0:5001", nil)

	select {}
}

func (webui *WEBUI) Exec(c *cli.Context) error {
	// WEBUI.Initialize(cfgPath, c)

	initLog.Traceln("args:", c.String("webuicfg"))
	args := webui.FilterCli(c)
	initLog.Traceln("filter: ", args)
	command := exec.Command("./webui", args...)

	webui.Initialize(c)

	stdout, err := command.StdoutPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	stderr, err := command.StderrPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	go func() {
		in := bufio.NewScanner(stderr)
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	go func() {
		if errCmd := command.Start(); errCmd != nil {
			fmt.Println("command.Start Fails!")
		}
		wg.Done()
	}()

	wg.Wait()

	return err
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
