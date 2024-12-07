package logger

import (
	"fmt"

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

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.LevelKey = "level"
	encoderConfig.EncodeLevel = CapitalColorLevelEncoder
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.StacktraceKey = ""

	config := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

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

func CapitalColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var color string
	switch l {
	case zapcore.DebugLevel:
		color = "\033[37m" // White
	case zapcore.InfoLevel:
		color = "\033[32m" // Green
	case zapcore.WarnLevel:
		color = "\033[33m" // Yellow
	case zapcore.ErrorLevel:
		color = "\033[31m" // Red
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		color = "\033[35m" // Magenta
	default:
		color = "\033[0m" // Reset
	}
	enc.AppendString(fmt.Sprintf("%s%s\033[0m", color, l.CapitalString()))
}
