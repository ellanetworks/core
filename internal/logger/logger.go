// Copyright 2024 Ella Networks

package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
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

func ConfigureLogging(systemLevel string, systemOutput string, systemFilePath string, auditOutput string, auditFilePath string) error {
	zapLevel, err := zapcore.ParseLevel(systemLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}
	atomicLevel.SetLevel(zapLevel)
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

	sysOutputPaths, err := resolveOutputPaths(systemOutput, systemFilePath)
	if err != nil {
		return fmt.Errorf("system logger configuration error: %v", err)
	}

	sysConfig := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console", // change to "json" if desired
		DisableCaller:    false,
		EncoderConfig:    encoderConfig,
		OutputPaths:      sysOutputPaths,
		ErrorOutputPaths: []string{"stderr"},
	}

	systemLogger, err := sysConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build system logger: %v", err)
	}

	auditOutputPaths, err := resolveOutputPaths(auditOutput, auditFilePath)
	if err != nil {
		return fmt.Errorf("audit logger configuration error: %v", err)
	}

	auditConfig := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		DisableCaller:    false,
		EncoderConfig:    encoderConfig,
		OutputPaths:      auditOutputPaths,
		ErrorOutputPaths: []string{"stderr"},
	}

	auditLogger, err := auditConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build audit logger: %v", err)
	}

	EllaLog = systemLogger.Sugar().With("component", "Ella")
	MetricsLog = systemLogger.Sugar().With("component", "Metrics")
	UtilLog = systemLogger.Sugar().With("component", "Util")
	DBLog = systemLogger.Sugar().With("component", "DB")
	AmfLog = systemLogger.Sugar().With("component", "AMF")
	AusfLog = systemLogger.Sugar().With("component", "AUSF")
	NmsLog = systemLogger.Sugar().With("component", "NMS")
	NssfLog = systemLogger.Sugar().With("component", "NSSF")
	PcfLog = systemLogger.Sugar().With("component", "PCF")
	SmfLog = systemLogger.Sugar().With("component", "SMF")
	UdmLog = systemLogger.Sugar().With("component", "UDM")
	UdrLog = systemLogger.Sugar().With("component", "UDR")
	UpfLog = systemLogger.Sugar().With("component", "UPF")

	AuditLog = auditLogger.Sugar().With("component", "Audit")
	return nil
}

func resolveOutputPaths(output string, filePath string) ([]string, error) {
	switch output {
	case "stdout":
		return []string{"stdout"}, nil
	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("file output specified but file path is empty")
		}
		if err := ensureFileWritable(filePath); err != nil {
			return nil, err
		}
		return []string{filePath}, nil
	default:
		return nil, fmt.Errorf("unknown output type: %s", output)
	}
}

func ensureFileWritable(filePath string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("unable to open file %s: %v", filePath, err)
	}
	return f.Close()
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
