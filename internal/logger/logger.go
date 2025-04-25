// Copyright 2024 Ella Networks

package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log         *zap.Logger
	EllaLog     *zap.Logger
	AuditLog    *zap.Logger
	MetricsLog  *zap.Logger
	DBLog       *zap.Logger
	AmfLog      *zap.Logger
	APILog      *zap.Logger
	SmfLog      *zap.Logger
	UdmLog      *zap.Logger
	UpfLog      *zap.Logger
	atomicLevel zap.AtomicLevel
)

const (
	FieldRanAddr     string = "ran_addr"
	FieldAmfUeNgapID string = "amf_ue_ngap_id"
	FieldSupi        string = "supi"
	FieldSuci        string = "suci"
)

// init sets up a default logger that writes to stdout.
// This configuration is used in tests and whenever ConfigureLogging is not called.
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

	// System logs for various components
	EllaLog = log.With(zap.String("component", "Ella"))
	MetricsLog = log.With(zap.String("component", "Metrics"))
	DBLog = log.With(zap.String("component", "DB"))
	AmfLog = log.With(zap.String("component", "AMF"))
	APILog = log.With(zap.String("component", "API"))
	SmfLog = log.With(zap.String("component", "SMF"))
	UdmLog = log.With(zap.String("component", "UDM"))
	UpfLog = log.With(zap.String("component", "UPF"))
	// Audit logger initially writes to stdout as well.
	AuditLog = log.With(zap.String("component", "Audit"))
}

// ConfigureLogging allows the user to reconfigure the logger.
// The caller specifies the log level and for each logger (system and audit) the output mode
// ("stdout", "file", or "both") and (if applicable) a file path.
func ConfigureLogging(systemLevel string, systemOutput string, systemFilePath string, auditOutput string, auditFilePath string) error {
	// Parse the desired system log level.
	zapLevel, err := zapcore.ParseLevel(systemLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}
	atomicLevel.SetLevel(zapLevel)

	// Create a shared encoder configuration.
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.LevelKey = "level"
	encoderConfig.EncodeLevel = CapitalColorLevelEncoder
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.MessageKey = "message"
	encoderConfig.StacktraceKey = ""

	// Determine output paths for system logs.
	sysOutputs, err := buildOutputPaths(systemOutput, systemFilePath)
	if err != nil {
		return fmt.Errorf("system logger: %w", err)
	}
	sysConfig := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		DisableCaller:    false,
		EncoderConfig:    encoderConfig,
		OutputPaths:      sysOutputs,
		ErrorOutputPaths: []string{"stderr"},
	}

	newSysLogger, err := sysConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build system logger: %w", err)
	}

	log = newSysLogger
	EllaLog = log.With(zap.String("component", "Ella"))
	MetricsLog = log.With(zap.String("component", "Metrics"))
	DBLog = log.With(zap.String("component", "DB"))
	AmfLog = log.With(zap.String("component", "AMF"))
	APILog = log.With(zap.String("component", "API"))
	SmfLog = log.With(zap.String("component", "SMF"))
	UdmLog = log.With(zap.String("component", "UDM"))
	UpfLog = log.With(zap.String("component", "UPF"))

	// Determine output paths for audit logs.
	auditOutputs, err := buildOutputPaths(auditOutput, auditFilePath)
	if err != nil {
		return fmt.Errorf("audit logger: %w", err)
	}
	auditConfig := zap.Config{
		Level:            atomicLevel,
		Development:      false,
		Encoding:         "console",
		DisableCaller:    false,
		EncoderConfig:    encoderConfig,
		OutputPaths:      auditOutputs,
		ErrorOutputPaths: []string{"stderr"},
	}

	auditLogger, err := auditConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build audit logger: %w", err)
	}
	AuditLog = auditLogger.With(zap.String("component", "Audit"))

	return nil
}

// buildOutputPaths builds a slice of output paths based on the output mode and file path.
// The mode can be "stdout", "file", or "both".
// If the mode is "file" or "both", filePath must be non-empty.
func buildOutputPaths(mode string, filePath string) ([]string, error) {
	switch mode {
	case "stdout":
		return []string{"stdout"}, nil
	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("file output selected but file path is empty")
		}
		return []string{filePath}, nil
	case "both":
		if filePath == "" {
			return nil, fmt.Errorf("both output selected but file path is empty")
		}
		return []string{"stdout", filePath}, nil
	default:
		// If mode is not recognized, default to stdout.
		return []string{"stdout"}, nil
	}
}

// CapitalColorLevelEncoder encodes the log level in color.
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

// LogAuditEvent logs an audit event to the audit logger.
func LogAuditEvent(action string, actor string, ip string, details string) {
	AuditLog.Info("Audit event", zap.String("action", action), zap.String("actor", actor), zap.String("ip", ip), zap.String("details", details))
}
