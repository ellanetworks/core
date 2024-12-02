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
