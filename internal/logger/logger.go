// Copyright 2024 Ella Networks

package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	EllaLog     *zap.SugaredLogger
	AuditLog    *zap.SugaredLogger
	UtilLog     *zap.SugaredLogger
	MetricsLog  *zap.SugaredLogger
	DBLog       *zap.SugaredLogger
	AmfLog      *zap.SugaredLogger
	AusfLog     *zap.SugaredLogger
	NmsLog      *zap.SugaredLogger
	NssfLog     *zap.SugaredLogger
	PcfLog      *zap.SugaredLogger
	SmfLog      *zap.SugaredLogger
	UdmLog      *zap.SugaredLogger
	UdrLog      *zap.SugaredLogger
	UpfLog      *zap.SugaredLogger
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

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.LevelKey = "level"
	encoderConfig.EncodeLevel = CapitalColorLevelEncoder
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.StacktraceKey = ""
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	config := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		DisableCaller:    false,
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
	MetricsLog = log.Sugar().With("component", "Metrics")
	UtilLog = log.Sugar().With("component", "Util")
	DBLog = log.Sugar().With("component", "DB")
	AmfLog = log.Sugar().With("component", "AMF")
	AusfLog = log.Sugar().With("component", "AUSF")
	NmsLog = log.Sugar().With("component", "NMS")
	NssfLog = log.Sugar().With("component", "NSSF")
	PcfLog = log.Sugar().With("component", "PCF")
	SmfLog = log.Sugar().With("component", "SMF")
	UdmLog = log.Sugar().With("component", "UDM")
	UdrLog = log.Sugar().With("component", "UDR")
	UpfLog = log.Sugar().With("component", "UPF")
	AuditLog = log.Sugar().With("component", "Audit")
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

func LogAuditEvent(action string, actor string, details string) {
	fields := []interface{}{
		"action", action,
		"actor", actor,
		"details", details,
	}
	AuditLog.Infow("audit event", fields...)
}
