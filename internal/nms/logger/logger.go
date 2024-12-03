package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	AppLog      *zap.SugaredLogger
	InitLog     *zap.SugaredLogger
	NMSLog      *zap.SugaredLogger
	ContextLog  *zap.SugaredLogger
	GinLog      *zap.SugaredLogger
	GrpcLog     *zap.SugaredLogger
	ConfigLog   *zap.SugaredLogger
	AuthLog     *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
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

	AppLog = log.Sugar().With("component", "NMS", "category", "App")
	InitLog = log.Sugar().With("component", "NMS", "category", "Init")
	NMSLog = log.Sugar().With("component", "NMS", "category", "NMS")
	ContextLog = log.Sugar().With("component", "NMS", "category", "Context")
	GinLog = log.Sugar().With("component", "NMS", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "NMS", "category", "GRPC")
	ConfigLog = log.Sugar().With("component", "NMS", "category", "CONFIG")
	AuthLog = log.Sugar().With("component", "NMS", "category", "Auth")
}

func GetLogger() *zap.Logger {
	return log
}

func SetLogLevel(level zapcore.Level) {
	InitLog.Infoln("set log level:", level)
	atomicLevel.SetLevel(level)
}
