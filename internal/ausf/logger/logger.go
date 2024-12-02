package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log                 *zap.Logger
	AppLog              *zap.SugaredLogger
	InitLog             *zap.SugaredLogger
	CfgLog              *zap.SugaredLogger
	UeAuthPostLog       *zap.SugaredLogger
	Auth5gAkaComfirmLog *zap.SugaredLogger
	EapAuthComfirmLog   *zap.SugaredLogger
	HandlerLog          *zap.SugaredLogger
	CallbackLog         *zap.SugaredLogger
	ProducerLog         *zap.SugaredLogger
	ContextLog          *zap.SugaredLogger
	ConsumerLog         *zap.SugaredLogger
	GinLog              *zap.SugaredLogger
	GrpcLog             *zap.SugaredLogger
	atomicLevel         zap.AtomicLevel
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

	AppLog = log.Sugar().With("component", "AUSF", "category", "App")
	InitLog = log.Sugar().With("component", "AUSF", "category", "Init")
	CfgLog = log.Sugar().With("component", "AUSF", "category", "CFG")
	UeAuthPostLog = log.Sugar().With("component", "AUSF", "category", "UeAuthPost")
	Auth5gAkaComfirmLog = log.Sugar().With("component", "AUSF", "category", "5gAkaAuth")
	EapAuthComfirmLog = log.Sugar().With("component", "AUSF", "category", "EapAkaAuth")
	HandlerLog = log.Sugar().With("component", "AUSF", "category", "Handler")
	CallbackLog = log.Sugar().With("component", "AUSF", "category", "Callback")
	ProducerLog = log.Sugar().With("component", "AUSF", "category", "Producer")
	ContextLog = log.Sugar().With("component", "AUSF", "category", "ctx")
	ConsumerLog = log.Sugar().With("component", "AUSF", "category", "Consumer")
	GinLog = log.Sugar().With("component", "AUSF", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "AUSF", "category", "GRPC")
}

func GetLogger() *zap.Logger {
	return log
}

// SetLogLevel: set the log level (panic|fatal|error|warn|info|debug)
func SetLogLevel(level zapcore.Level) {
	InitLog.Infoln("set log level:", level)
	atomicLevel.SetLevel(level)
}
