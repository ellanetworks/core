package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	AppLog      *zap.SugaredLogger
	InitLog     *zap.SugaredLogger
	CfgLog      *zap.SugaredLogger
	HandlerLog  *zap.SugaredLogger
	DataRepoLog *zap.SugaredLogger
	UtilLog     *zap.SugaredLogger
	HttpLog     *zap.SugaredLogger
	ConsumerLog *zap.SugaredLogger
	GinLog      *zap.SugaredLogger
	GrpcLog     *zap.SugaredLogger
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

	AppLog = log.Sugar().With("component", "UDR", "category", "App")
	InitLog = log.Sugar().With("component", "UDR", "category", "Init")
	CfgLog = log.Sugar().With("component", "UDR", "category", "CFG")
	HandlerLog = log.Sugar().With("component", "UDR", "category", "HDLR")
	DataRepoLog = log.Sugar().With("component", "UDR", "category", "DRepo")
	UtilLog = log.Sugar().With("component", "UDR", "category", "Util")
	HttpLog = log.Sugar().With("component", "UDR", "category", "HTTP")
	ConsumerLog = log.Sugar().With("component", "UDR", "category", "Consumer")
	GinLog = log.Sugar().With("component", "UDR", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "UDR", "category", "GRPC")
}

func GetLogger() *zap.Logger {
	return log
}

func SetLogLevel(level zapcore.Level) {
	CfgLog.Infoln("set log level:", level)
	atomicLevel.SetLevel(level)
}
