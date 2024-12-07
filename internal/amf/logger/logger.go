package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	AppLog      *zap.SugaredLogger
	InitLog     *zap.SugaredLogger
	CfgLog      *zap.SugaredLogger
	ContextLog  *zap.SugaredLogger
	DataRepoLog *zap.SugaredLogger
	NgapLog     *zap.SugaredLogger
	HandlerLog  *zap.SugaredLogger
	HttpLog     *zap.SugaredLogger
	GmmLog      *zap.SugaredLogger
	MtLog       *zap.SugaredLogger
	ProducerLog *zap.SugaredLogger
	LocationLog *zap.SugaredLogger
	CommLog     *zap.SugaredLogger
	CallbackLog *zap.SugaredLogger
	UtilLog     *zap.SugaredLogger
	NasLog      *zap.SugaredLogger
	ConsumerLog *zap.SugaredLogger
	EeLog       *zap.SugaredLogger
	GinLog      *zap.SugaredLogger
	GrpcLog     *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
)

const (
	FieldRanAddr     string = "ran_addr"
	FieldRanId       string = "ran_id"
	FieldAmfUeNgapID string = "amf_ue_ngap_id"
	FieldSupi        string = "supi"
	FieldSuci        string = "suci"
)

func init() {
	atomicLevel = zap.NewAtomicLevelAt(zap.InfoLevel)

	// Define a custom encoder with colorized levels
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.LevelKey = "level"
	encoderConfig.EncodeLevel = CapitalColorLevelEncoder // Use custom colorized levels
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

	AppLog = log.Sugar().With("component", "AMF", "category", "App")
	InitLog = log.Sugar().With("component", "AMF", "category", "Init")
	CfgLog = log.Sugar().With("component", "AMF", "category", "CFG")
	ContextLog = log.Sugar().With("component", "AMF", "category", "Context")
	DataRepoLog = log.Sugar().With("component", "AMF", "category", "DBRepo")
	NgapLog = log.Sugar().With("component", "AMF", "category", "NGAP")
	HandlerLog = log.Sugar().With("component", "AMF", "category", "Handler")
	HttpLog = log.Sugar().With("component", "AMF", "category", "HTTP")
	GmmLog = log.Sugar().With("component", "AMF", "category", "GMM")
	MtLog = log.Sugar().With("component", "AMF", "category", "MT")
	ProducerLog = log.Sugar().With("component", "AMF", "category", "Producer")
	LocationLog = log.Sugar().With("component", "AMF", "category", "LocInfo")
	CommLog = log.Sugar().With("component", "AMF", "category", "Comm")
	CallbackLog = log.Sugar().With("component", "AMF", "category", "Callback")
	UtilLog = log.Sugar().With("component", "AMF", "category", "Util")
	NasLog = log.Sugar().With("component", "AMF", "category", "NAS")
	ConsumerLog = log.Sugar().With("component", "AMF", "category", "Consumer")
	EeLog = log.Sugar().With("component", "AMF", "category", "EventExposure")
	GinLog = log.Sugar().With("component", "AMF", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "AMF", "category", "GRPC")
}

func GetLogger() *zap.Logger {
	return log
}

// SetLogLevel: set the log level (panic|fatal|error|warn|info|debug)
func SetLogLevel(level zapcore.Level) {
	CfgLog.Infoln("set log level:", level)
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
