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
	DataRepoLog *zap.SugaredLogger
	GsmLog      *zap.SugaredLogger
	PfcpLog     *zap.SugaredLogger
	PduSessLog  *zap.SugaredLogger
	CtxLog      *zap.SugaredLogger
	ConsumerLog *zap.SugaredLogger
	GinLog      *zap.SugaredLogger
	GrpcLog     *zap.SugaredLogger
	ProducerLog *zap.SugaredLogger
	UPNodeLog   *zap.SugaredLogger
	FsmLog      *zap.SugaredLogger
	TxnFsmLog   *zap.SugaredLogger
	QosLog      *zap.SugaredLogger
	KafkaLog    *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
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

	AppLog = log.Sugar().With("component", "SMF", "category", "App")
	InitLog = log.Sugar().With("component", "SMF", "category", "Init")
	CfgLog = log.Sugar().With("component", "SMF", "category", "CFG")
	DataRepoLog = log.Sugar().With("component", "SMF", "category", "DRepo")
	PfcpLog = log.Sugar().With("component", "SMF", "category", "PFCP")
	PduSessLog = log.Sugar().With("component", "SMF", "category", "PduSess")
	GsmLog = log.Sugar().With("component", "SMF", "category", "GSM")
	CtxLog = log.Sugar().With("component", "SMF", "category", "CTX")
	ConsumerLog = log.Sugar().With("component", "SMF", "category", "Consumer")
	GinLog = log.Sugar().With("component", "SMF", "category", "GIN")
	GrpcLog = log.Sugar().With("component", "SMF", "category", "GRPC")
	ProducerLog = log.Sugar().With("component", "SMF", "category", "Producer")
	UPNodeLog = log.Sugar().With("component", "SMF", "category", "UPNode")
	FsmLog = log.Sugar().With("component", "SMF", "category", "Fsm")
	TxnFsmLog = log.Sugar().With("component", "SMF", "category", "TxnFsm")
	QosLog = log.Sugar().With("component", "SMF", "category", "QosFsm")
	KafkaLog = log.Sugar().With("component", "SMF", "category", "Kafka")
}

func GetLogger() *zap.Logger {
	return log
}

// SetLogLevel: set the log level (panic|fatal|error|warn|info|debug)
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
