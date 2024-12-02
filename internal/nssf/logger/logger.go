package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log               *zap.Logger
	AppLog            *zap.SugaredLogger
	CfgLog            *zap.SugaredLogger
	ContextLog        *zap.SugaredLogger
	FactoryLog        *zap.SugaredLogger
	HandlerLog        *zap.SugaredLogger
	InitLog           *zap.SugaredLogger
	Nsselection       *zap.SugaredLogger
	Nssaiavailability *zap.SugaredLogger
	Util              *zap.SugaredLogger
	ConsumerLog       *zap.SugaredLogger
	GinLog            *zap.SugaredLogger
	GrpcLog           *zap.SugaredLogger
	atomicLevel       zap.AtomicLevel
)

func init() {
	atomicLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	config := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.StacktraceKey = ""

	var err error
	log, err = config.Build()
	if err != nil {
		panic(err)
	}

	AppLog = log.Sugar().With("component", "NSSF", "category", "App")
	ContextLog = log.Sugar().With("component", "NSSF", "category", "CTX")
	FactoryLog = log.Sugar().With("component", "NSSF", "category", "Factory")
	HandlerLog = log.Sugar().With("component", "NSSF", "category", "HDLR")
	InitLog = log.Sugar().With("component", "NSSF", "category", "Init")
	CfgLog = log.Sugar().With("component", "NSSF", "category", "CFG")
	Nsselection = log.Sugar().With("component", "NSSF", "category", "NsSelect")
	Nssaiavailability = log.Sugar().With("component", "NSSF", "category", "NssaiAvail")
	Util = log.Sugar().With("component", "NSSF", "category", "Util")
	ConsumerLog = log.Sugar().With("component", "NSSF", "category", "Consumer")
	GinLog = log.Sugar().With("component", "NSSF", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "NSSF", "category", "GRPC")
}

func GetLogger() *zap.Logger {
	return log
}

// SetLogLevel: set the log level (panic|fatal|error|warn|info|debug)
func SetLogLevel(level zapcore.Level) {
	InitLog.Infoln("set log level:", level)
	atomicLevel.SetLevel(level)
}
