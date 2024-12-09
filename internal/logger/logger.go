package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	EllaLog     *zap.SugaredLogger
	DBLog       *zap.SugaredLogger
	AmfLog      *zap.SugaredLogger
	AusfLog     *zap.SugaredLogger
	NMSLog      *zap.SugaredLogger
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

	EllaLog = log.Sugar().With("component", "Ella")
	DBLog = log.Sugar().With("component", "DB")
	AmfLog = log.Sugar().With("component", "AMF")
	AusfLog = log.Sugar().With("component", "AUSF")
	NMSLog = log.Sugar().With("component", "NMS")
}

func GetLogger() *zap.Logger {
	return log
}

// SetLogLevel: set the log level (panic|fatal|error|warn|info|debug)
func SetLogLevel(level zapcore.Level) {
	EllaLog.Infoln("set log level:", level)
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
