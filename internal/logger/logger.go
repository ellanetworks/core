// Copyright 2024 Ella Networks

package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"go.opentelemetry.io/otel/trace"
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
	SessionsLog *zap.Logger
	NetworkLog  *zap.Logger

	atomicLevel zap.AtomicLevel

	auditDBSink zapcore.WriteSyncer

	dbInstance dbwriter.DBWriter
)

// Default: console only, info level.
func init() {
	_ = ConfigureLogging("info", "stdout", "", "stdout", "")
}

// ConfigureLogging builds loggers with a simple tee of console + optional file.
// Audit logs also go to DB if SetAuditDBWriter was called.
func ConfigureLogging(systemLevel, systemOutput, systemFilePath, auditOutput, auditFilePath string) error {
	zl, err := zapcore.ParseLevel(systemLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}

	atomicLevel = zap.NewAtomicLevelAt(zl)

	jsonEnc := zapcore.NewJSONEncoder(jsonEncoderConfig())

	sysCores, err := makeCores(systemOutput, systemFilePath, jsonEnc)
	if err != nil {
		return fmt.Errorf("system logger: %w", err)
	}

	sysLogger := zap.New(zapcore.NewTee(sysCores...), zap.AddCaller())

	auditCores, err := makeCores(auditOutput, auditFilePath, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	if auditDBSink != nil {
		auditCores = append(auditCores, zapcore.NewCore(jsonEnc, auditDBSink, atomicLevel))
	}

	auditLogger := zap.New(zapcore.NewTee(auditCores...), zap.AddCaller())

	networkCores, err := makeCores(systemOutput, systemFilePath, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	networkLogger := zap.New(zapcore.NewTee(networkCores...), zap.AddCaller())

	// Swap roots
	log = sysLogger
	AuditLog = auditLogger.With(zap.String("component", "Audit"))
	NetworkLog = networkLogger.With(zap.String("component", "Network"))

	// Component children from system logger
	EllaLog = log.With(zap.String("component", "Ella"))
	MetricsLog = log.With(zap.String("component", "Metrics"))
	DBLog = log.With(zap.String("component", "DB"))
	AmfLog = log.With(zap.String("component", "AMF"))
	APILog = log.With(zap.String("component", "API"))
	SmfLog = log.With(zap.String("component", "SMF"))
	UdmLog = log.With(zap.String("component", "UDM"))
	UpfLog = log.With(zap.String("component", "UPF"))
	SessionsLog = log.With(zap.String("component", "Sessions"))

	return nil
}

func SetDb(db dbwriter.DBWriter) {
	dbInstance = db
}

// WithTrace returns a logger enriched with traceID and spanID fields
// extracted from the given context. If the context has no active span,
// the original logger is returned unchanged.
func WithTrace(ctx context.Context, l *zap.Logger) *zap.Logger {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return l
	}

	return l.With(
		zap.String("traceID", sc.TraceID().String()),
		zap.String("spanID", sc.SpanID().String()),
	)
}

// makeCores returns JSON cores for stdout and optional file output.
func makeCores(mode, filePath string, enc zapcore.Encoder) ([]zapcore.Core, error) {
	cores := []zapcore.Core{
		zapcore.NewCore(enc, zapcore.Lock(os.Stdout), atomicLevel),
	}

	switch mode {
	case "stdout":
		// nothing else
	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("file output selected but file path is empty")
		}

		ws, err := openFileSync(filePath)
		if err != nil {
			return nil, err
		}

		cores = append(cores, zapcore.NewCore(enc, ws, atomicLevel))
	default:
	}

	return cores, nil
}

// openFileSync opens/creates a file and returns a WriteSyncer with a lock.
func openFileSync(path string) (zapcore.WriteSyncer, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec: G304
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	return zapcore.Lock(zapcore.AddSync(f)), nil
}

func jsonEncoderConfig() zapcore.EncoderConfig {
	enc := zap.NewProductionEncoderConfig()
	enc.TimeKey = "ts"
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.LevelKey = "level"
	enc.EncodeLevel = zapcore.LowercaseLevelEncoder
	enc.CallerKey = "caller"
	enc.EncodeCaller = zapcore.ShortCallerEncoder
	enc.MessageKey = "msg"
	enc.StacktraceKey = ""

	return enc
}

// LogAuditEvent logs an audit event to the audit logger.
func LogAuditEvent(ctx context.Context, action, actor, ip, details string) {
	WithTrace(ctx, AuditLog).Info("Audit event",
		zap.String("action", action),
		zap.String("actor", actor),
		zap.String("ip", ip),
		zap.String("details", details),
	)

	if dbInstance == nil {
		NetworkLog.Warn("dbInstance is nil, cannot log network event to database")
		return
	}

	err := dbInstance.InsertAuditLog(ctx, &dbwriter.AuditLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "INFO",
		Actor:     actor,
		Action:    action,
		IP:        ip,
		Details:   details,
	})
	if err != nil {
		AuditLog.Warn("failed to insert audit log",
			zap.Error(err),
		)
	}
}

type LogDirection string

const (
	DirectionInbound  LogDirection = "inbound"
	DirectionOutbound LogDirection = "outbound"
)

type NetworkProtocol string

const (
	NGAPNetworkProtocol NetworkProtocol = "NGAP"
)

func LogNetworkEvent(
	ctx context.Context,
	protocol NetworkProtocol,
	messageType string,
	dir LogDirection,
	localAddress string,
	remoteAddress string,
	rawBytes []byte,
) {
	if NetworkLog == nil {
		return
	}

	if messageType == "" {
		EllaLog.Warn("attempted to log empty network message type",
			zap.String("protocol", string(protocol)),
			zap.String("dir", string(dir)),
			zap.String("local_address", localAddress),
			zap.String("remote_address", remoteAddress),
		)

		return
	}

	WithTrace(ctx, NetworkLog).Info("network_event",
		zap.String("protocol", string(protocol)),
		zap.String("message_type", messageType),
		zap.String("direction", string(dir)),
		zap.String("local_address", localAddress),
		zap.String("remote_address", remoteAddress),
	)

	if dbInstance == nil {
		NetworkLog.Warn("dbInstance is nil, cannot log network event to database")
		return
	}

	err := dbInstance.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Protocol:      string(protocol),
		MessageType:   messageType,
		Direction:     string(dir),
		LocalAddress:  localAddress,
		RemoteAddress: remoteAddress,
		Raw:           rawBytes,
	})
	if err != nil {
		NetworkLog.Warn("failed to insert radio event",
			zap.Error(err),
		)
	}
}
