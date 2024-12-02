package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log                    *zap.Logger
	AppLog                 *zap.SugaredLogger
	InitLog                *zap.SugaredLogger
	CfgLog                 *zap.SugaredLogger
	HandlerLog             *zap.SugaredLogger
	Bdtpolicylog           *zap.SugaredLogger
	PolicyAuthorizationlog *zap.SugaredLogger
	AMpolicylog            *zap.SugaredLogger
	SMpolicylog            *zap.SugaredLogger
	Consumerlog            *zap.SugaredLogger
	UtilLog                *zap.SugaredLogger
	CallbackLog            *zap.SugaredLogger
	OamLog                 *zap.SugaredLogger
	CtxLog                 *zap.SugaredLogger
	ConsumerLog            *zap.SugaredLogger
	GinLog                 *zap.SugaredLogger
	GrpcLog                *zap.SugaredLogger
	NotifyEventLog         *zap.SugaredLogger
	ProducerLog            *zap.SugaredLogger
	atomicLevel            zap.AtomicLevel
)

const (
	FieldSupi string = "supi"
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

	AppLog = log.Sugar().With("component", "PCF", "category", "App")
	InitLog = log.Sugar().With("component", "PCF", "category", "Init")
	CfgLog = log.Sugar().With("component", "PCF", "category", "CFG")
	HandlerLog = log.Sugar().With("component", "PCF", "category", "Handler")
	Bdtpolicylog = log.Sugar().With("component", "PCF", "category", "Bdtpolicy")
	AMpolicylog = log.Sugar().With("component", "PCF", "category", "Ampolicy")
	PolicyAuthorizationlog = log.Sugar().With("component", "PCF", "category", "PolicyAuth")
	SMpolicylog = log.Sugar().With("component", "PCF", "category", "SMpolicy")
	UtilLog = log.Sugar().With("component", "PCF", "category", "Util")
	CallbackLog = log.Sugar().With("component", "PCF", "category", "Callback")
	Consumerlog = log.Sugar().With("component", "PCF", "category", "Consumer")
	OamLog = log.Sugar().With("component", "PCF", "category", "OAM")
	CtxLog = log.Sugar().With("component", "PCF", "category", "Context")
	ConsumerLog = log.Sugar().With("component", "PCF", "category", "Consumer")
	GinLog = log.Sugar().With("component", "PCF", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "PCF", "category", "GRPC")
	NotifyEventLog = log.Sugar().With("component", "PCF", "category", "NotifyEvent")
	ProducerLog = log.Sugar().With("component", "PCF", "category", "Producer")
}

func GetLogger() *zap.Logger {
	return log
}

func SetLogLevel(level zapcore.Level) {
	InitLog.Infoln("set log level:", level)
	atomicLevel.SetLevel(level)
}
