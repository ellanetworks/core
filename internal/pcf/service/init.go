package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/ampolicy"
	"github.com/yeastengine/ella/internal/pcf/bdtpolicy"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/internal/notifyevent"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/oam"
	"github.com/yeastengine/ella/internal/pcf/policyauthorization"
	"github.com/yeastengine/ella/internal/pcf/smpolicy"
	"github.com/yeastengine/ella/internal/pcf/uepolicy"
	"github.com/yeastengine/ella/internal/pcf/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type PCF struct{}

func (pcf *PCF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	pcf.setLogLevel()
}

func (pcf *PCF) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.PcfConfig.Logger.PCF.DebugLevel); err != nil {
		logger.InitLog.Warnf("PCF Log level [%s] is invalid, set to [info] level",
			factory.PcfConfig.Logger.PCF.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("PCF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (pcf *PCF) Start() {
	router := logger_util.NewGinWithZap(logger.GinLog)

	bdtpolicy.AddService(router)
	smpolicy.AddService(router)
	ampolicy.AddService(router)
	uepolicy.AddService(router)
	policyauthorization.AddService(router)
	oam.AddService(router)

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

	if err := notifyevent.RegisterNotifyDispatcher(); err != nil {
		logger.InitLog.Error("Register NotifyDispatcher Error")
	}

	self := context.PCF_Self()
	util.InitpcfContext(self)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		pcf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, "/var/log/pcf.log", router)
	if server == nil {
		logger.InitLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		logger.InitLog.Warnf("Initialize HTTP server: +%v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		logger.InitLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (pcf *PCF) Terminate() {
	logger.InitLog.Infof("PCF terminated")
}
