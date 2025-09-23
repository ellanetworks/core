// Copyright 2024 Ella Networks

package logger

import (
	"encoding/json"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log           *zap.Logger
	EllaLog       *zap.Logger
	AuditLog      *zap.Logger
	MetricsLog    *zap.Logger
	DBLog         *zap.Logger
	AmfLog        *zap.Logger
	APILog        *zap.Logger
	SmfLog        *zap.Logger
	UdmLog        *zap.Logger
	UpfLog        *zap.Logger
	SessionsLog   *zap.Logger
	SubscriberLog *zap.Logger
	RadioLog      *zap.Logger

	atomicLevel zap.AtomicLevel

	auditDBSink      zapcore.WriteSyncer
	subscriberDBSink zapcore.WriteSyncer
	radioDBSink      zapcore.WriteSyncer
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

	consoleEnc := zapcore.NewConsoleEncoder(devConsoleEncoderConfig())
	jsonEnc := zapcore.NewJSONEncoder(prodJSONEncoderConfig())

	sysCores, err := makeCores(systemOutput, systemFilePath, consoleEnc, jsonEnc)
	if err != nil {
		return fmt.Errorf("system logger: %w", err)
	}
	sysLogger := zap.New(zapcore.NewTee(sysCores...), zap.AddCaller())

	auditCores, err := makeCores(auditOutput, auditFilePath, consoleEnc, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	if auditDBSink != nil {
		auditCores = append(auditCores, zapcore.NewCore(jsonEnc, auditDBSink, atomicLevel))
	}

	auditLogger := zap.New(zapcore.NewTee(auditCores...), zap.AddCaller())

	subscriberCores, err := makeCores(systemOutput, systemFilePath, consoleEnc, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	if subscriberDBSink != nil {
		subscriberCores = append(subscriberCores, zapcore.NewCore(jsonEnc, subscriberDBSink, atomicLevel))
	}

	subscriberLogger := zap.New(zapcore.NewTee(subscriberCores...), zap.AddCaller())

	radioCores, err := makeCores(systemOutput, systemFilePath, consoleEnc, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	if radioDBSink != nil {
		radioCores = append(radioCores, zapcore.NewCore(jsonEnc, radioDBSink, atomicLevel))
	}

	radioLogger := zap.New(zapcore.NewTee(radioCores...), zap.AddCaller())

	// Swap roots
	log = sysLogger
	AuditLog = auditLogger.With(zap.String("component", "Audit"))
	SubscriberLog = subscriberLogger.With(zap.String("component", "Subscriber"))
	RadioLog = radioLogger.With(zap.String("component", "Radio"))

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

// SetAuditDBWriter registers a function that persists one JSON-encoded audit entry.
// This function should be called after the DB is ready.
func SetAuditDBWriter(writeFn func([]byte) error) {
	if writeFn == nil {
		auditDBSink = nil
		return
	}
	ws := zapcore.AddSync(funcWriteSyncer{write: writeFn})
	auditDBSink = ws

	// If the audit logger already exists, attach the DB core now.
	if AuditLog != nil {
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(prodJSONEncoderConfig()),
			auditDBSink,
			atomicLevel,
		)
		AuditLog = AuditLog.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewTee(c, core)
		}))
	}
}

func SetSubscriberDBWriter(writeFn func([]byte) error) {
	if writeFn == nil {
		subscriberDBSink = nil
		return
	}
	ws := zapcore.AddSync(funcWriteSyncer{write: writeFn})
	subscriberDBSink = ws

	// If the subscriber logger already exists, attach the DB core now.
	if SubscriberLog != nil {
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(prodJSONEncoderConfig()),
			subscriberDBSink,
			atomicLevel,
		)
		SubscriberLog = SubscriberLog.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewTee(c, core)
		}))
	}
}

func SetRadioDBWriter(writeFn func([]byte) error) {
	if writeFn == nil {
		radioDBSink = nil
		return
	}
	ws := zapcore.AddSync(funcWriteSyncer{write: writeFn})
	radioDBSink = ws

	// If the radio logger already exists, attach the DB core now.
	if RadioLog != nil {
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(prodJSONEncoderConfig()),
			radioDBSink,
			atomicLevel,
		)
		RadioLog = RadioLog.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewTee(c, core)
		}))
	}
}

// makeCores returns console core (+ file core if requested).
func makeCores(mode, filePath string, consoleEnc, jsonEnc zapcore.Encoder) ([]zapcore.Core, error) {
	cores := []zapcore.Core{
		zapcore.NewCore(consoleEnc, zapcore.Lock(os.Stdout), atomicLevel),
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
		cores = append(cores, zapcore.NewCore(jsonEnc, ws, atomicLevel))
	case "both":
		if filePath == "" {
			return nil, fmt.Errorf("both output selected but file path is empty")
		}
		ws, err := openFileSync(filePath)
		if err != nil {
			return nil, err
		}
		cores = append(cores, zapcore.NewCore(jsonEnc, ws, atomicLevel))
	default:
		// default to stdout only
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

func devConsoleEncoderConfig() zapcore.EncoderConfig {
	enc := zap.NewDevelopmentEncoderConfig()
	enc.TimeKey = "timestamp"
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.LevelKey = "level"
	enc.EncodeLevel = CapitalColorLevelEncoder // keep colors
	enc.CallerKey = "caller"
	enc.EncodeCaller = zapcore.ShortCallerEncoder
	enc.MessageKey = "message"
	enc.StacktraceKey = ""
	return enc
}

func prodJSONEncoderConfig() zapcore.EncoderConfig {
	enc := zap.NewProductionEncoderConfig()
	enc.TimeKey = "timestamp"
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.LevelKey = "level"
	enc.EncodeLevel = zapcore.LowercaseLevelEncoder
	enc.CallerKey = "caller"
	enc.EncodeCaller = zapcore.ShortCallerEncoder
	enc.MessageKey = "message"
	enc.StacktraceKey = ""
	return enc
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
func LogAuditEvent(action, actor, ip, details string) {
	AuditLog.Info("Audit event",
		zap.String("action", action),
		zap.String("actor", actor),
		zap.String("ip", ip),
		zap.String("details", details),
	)
}

type SubscriberEvent string

const (
	// Access events
	SubscriberRegistrationRequest                     SubscriberEvent = "Registration Request"
	SubscriberInitialRegistration                     SubscriberEvent = "Initial Registration"
	SubscriberMobilityAndPeriodicRegistrationUpdating SubscriberEvent = "Mobility and Periodic Registration Updating"
	SubscriberIdentityResponse                        SubscriberEvent = "Identity Response"
	SubscriberNotificationResponse                    SubscriberEvent = "Notification Response"
	SubscriberConfigurationUpdateComplete             SubscriberEvent = "Configuration Update Complete"
	SubscriberServiceRequest                          SubscriberEvent = "Service Request"
	SubscriberAuthenticationResponse                  SubscriberEvent = "Authentication Response"
	SubscriberAuthenticationFailure                   SubscriberEvent = "Authentication Failure"
	SubscriberRegistrationComplete                    SubscriberEvent = "Registration Complete"
	SubscriberSecurityModeComplete                    SubscriberEvent = "Security Mode Complete"
	SubscriberSecurityModeReject                      SubscriberEvent = "Security Mode Reject"
	SubscriberDeregistrationRequest                   SubscriberEvent = "Deregistration Request"
	SubscriberDeregistrationAccept                    SubscriberEvent = "Deregistration Accept"
	SubscriberStatus5GMM                              SubscriberEvent = "Status 5GMM"
	SubscriberAuthenticationError                     SubscriberEvent = "Authentication Error"

	// Session events
	SubscriberPduSessionEstablishmentRequest SubscriberEvent = "PDU Session Establishment Request"
	SubscriberPduSessionEstablishmentReject  SubscriberEvent = "PDU Session Establishment Reject"
	SubscriberPduSessionEstablishmentAccept  SubscriberEvent = "PDU Session Establishment Accept"
)

func LogSubscriberEvent(event SubscriberEvent, imsi string, fields ...zap.Field) {
	if SubscriberLog == nil {
		return
	}

	if event == "" {
		EllaLog.Warn("attempted to log empty subscriber event",
			zap.String("imsi", imsi),
			zap.Any("fields", fields),
		)
		return
	}

	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	var detailsStr string

	reserved := map[string]struct{}{
		"event": {}, "imsi": {}, "timestamp": {}, "level": {},
		"component": {}, "caller": {}, "message": {},
	}

	if raw, ok := enc.Fields["details"]; ok {
		switch v := raw.(type) {
		case string:
			detailsStr = v
		default:
			if b, err := json.Marshal(v); err == nil {
				detailsStr = string(b)
			}
		}
		delete(enc.Fields, "details")
	}

	if detailsStr == "" {
		agg := make(map[string]any, len(enc.Fields))
		for k, v := range enc.Fields {
			if _, isReserved := reserved[k]; !isReserved {
				agg[k] = v
			}
		}
		if len(agg) > 0 {
			if b, err := json.Marshal(agg); err == nil {
				detailsStr = string(b)
			}
		}
	}

	// Optional safety: cap the size
	const maxDetails = 4096
	if len(detailsStr) > maxDetails {
		detailsStr = detailsStr[:maxDetails] + "...(truncated)"
	}

	// Emit a single, consistent log line. DB reader already expects details as string.
	SubscriberLog.Info("subscriber_event",
		zap.String("event", string(event)),
		zap.String("imsi", imsi),
		zap.String("details", detailsStr),
	)
}

type RadioEvent string

const (
	RadioNGSetupRequest     RadioEvent = "NG Setup Request"
	RadioUplinkNASTransport RadioEvent = "Uplink NAS Transport"
	RadioNGReset            RadioEvent = "NG Reset"
)

func LogRadioEvent(event RadioEvent, id string, fields ...zap.Field) {
	if RadioLog == nil {
		return
	}

	if event == "" {
		EllaLog.Warn("attempted to log empty radio event",
			zap.String("id", id),
			zap.Any("fields", fields),
		)
		return
	}

	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	var detailsStr string

	reserved := map[string]struct{}{
		"event": {}, "id": {}, "timestamp": {}, "level": {},
		"component": {}, "caller": {}, "message": {},
	}

	if raw, ok := enc.Fields["details"]; ok {
		switch v := raw.(type) {
		case string:
			detailsStr = v
		default:
			if b, err := json.Marshal(v); err == nil {
				detailsStr = string(b)
			}
		}
		delete(enc.Fields, "details")
	}

	if detailsStr == "" {
		agg := make(map[string]any, len(enc.Fields))
		for k, v := range enc.Fields {
			if _, isReserved := reserved[k]; !isReserved {
				agg[k] = v
			}
		}
		if len(agg) > 0 {
			if b, err := json.Marshal(agg); err == nil {
				detailsStr = string(b)
			}
		}
	}

	// Optional safety: cap the size
	const maxDetails = 4096
	if len(detailsStr) > maxDetails {
		detailsStr = detailsStr[:maxDetails] + "...(truncated)"
	}

	// Emit a single, consistent log line. DB reader already expects details as string.
	RadioLog.Info("radio_event",
		zap.String("event", string(event)),
		zap.String("id", id),
		zap.String("details", detailsStr),
	)
}

type funcWriteSyncer struct {
	write func([]byte) error
}

func (f funcWriteSyncer) Write(p []byte) (int, error) {
	if err := f.write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}
