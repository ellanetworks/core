// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
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
	MmeLog      *zap.Logger
	APILog      *zap.Logger
	SmfLog      *zap.Logger
	UpfLog      *zap.Logger
	SessionsLog *zap.Logger
	NetworkLog  *zap.Logger
	RaftLog     *zap.Logger
	LmfLog      *zap.Logger
	BgpLog      *zap.Logger

	atomicLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	filesMu   sync.Mutex
	openFiles []*os.File

	dbInstance dbwriter.DBWriter
)

// Default: console only, info level.
func init() {
	_ = ConfigureLogging("info", "stdout", "", "stdout", "")
}

func ConfigureLogging(systemLevel, systemOutput, systemFilePath, auditOutput, auditFilePath string) error {
	zl, err := zapcore.ParseLevel(systemLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}

	jsonEnc := zapcore.NewJSONEncoder(jsonEncoderConfig())

	sysCores, sysFiles, err := makeCores(systemOutput, systemFilePath, jsonEnc)
	if err != nil {
		return fmt.Errorf("system logger: %w", err)
	}

	auditCores, auditFiles, err := makeCores(auditOutput, auditFilePath, jsonEnc)
	if err != nil {
		closeAll(sysFiles)
		return fmt.Errorf("audit logger: %w", err)
	}

	atomicLevel.SetLevel(zl)

	log = zap.New(newDedupeCore(zapcore.NewTee(sysCores...)), zap.AddCaller())
	auditRoot := zap.New(newDedupeCore(zapcore.NewTee(auditCores...)), zap.AddCaller())

	AuditLog = auditRoot.Named("Audit")
	NetworkLog = Scope("Network")
	EllaLog = Scope("Ella")
	MetricsLog = Scope("Metrics")
	DBLog = Scope("DB")
	AmfLog = Scope("AMF")
	MmeLog = Scope("MME")
	APILog = Scope("API")
	SmfLog = Scope("SMF")
	UpfLog = Scope("UPF")
	SessionsLog = Scope("Sessions")
	RaftLog = Scope("Raft")
	LmfLog = Scope("LMF")
	BgpLog = Scope("BGP")

	zap.RedirectStdLog(EllaLog)

	closeAll(trackFiles(append(sysFiles, auditFiles...)))

	return nil
}

func SetLevel(level string) error {
	zl, err := zapcore.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %v", err)
	}

	atomicLevel.SetLevel(zl)

	return nil
}

func Close() error {
	_ = log.Sync()
	_ = AuditLog.Sync()

	atomicLevel.SetLevel(zapcore.FatalLevel + 1)

	var err error

	for _, f := range trackFiles(nil) {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	return err
}

func trackFiles(files []*os.File) []*os.File {
	filesMu.Lock()
	defer filesMu.Unlock()

	prev := openFiles
	openFiles = files

	return prev
}

func closeAll(files []*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}

func Scope(name string) *zap.Logger {
	return log.Named(name)
}

type stdLogWriter struct {
	zap *zap.Logger
}

func (w stdLogWriter) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	if line == "" {
		return len(p), nil
	}

	w.zap.Warn("HTTP server error", zap.String("error", line))

	return len(p), nil
}

func StdLogger(l *zap.Logger) *stdlog.Logger {
	return stdlog.New(stdLogWriter{zap: l}, "", 0)
}

func SetDb(db dbwriter.DBWriter) {
	dbInstance = db
}

func SwapSystemCore(core zapcore.Core) func() {
	prev := log

	log = zap.New(newDedupeCore(core), zap.AddCaller())

	restore := namedScopes()

	for _, s := range restore {
		*s.target = Scope(s.name)
	}

	return func() {
		log = prev

		for _, s := range restore {
			*s.target = Scope(s.name)
		}
	}
}

type namedScope struct {
	target **zap.Logger
	name   string
}

func namedScopes() []namedScope {
	return []namedScope{
		{&NetworkLog, "Network"},
		{&EllaLog, "Ella"},
		{&MetricsLog, "Metrics"},
		{&DBLog, "DB"},
		{&AmfLog, "AMF"},
		{&MmeLog, "MME"},
		{&APILog, "API"},
		{&SmfLog, "SMF"},
		{&UpfLog, "UPF"},
		{&SessionsLog, "Sessions"},
		{&RaftLog, "Raft"},
		{&LmfLog, "LMF"},
		{&BgpLog, "BGP"},
	}
}

// makeCores returns JSON cores for stdout and optional file output.
func makeCores(mode, filePath string, enc zapcore.Encoder) ([]zapcore.Core, []*os.File, error) {
	cores := []zapcore.Core{
		zapcore.NewCore(enc, zapcore.Lock(os.Stdout), atomicLevel),
	}

	switch mode {
	case "stdout":
		// nothing else
	case "file":
		if filePath == "" {
			return nil, nil, fmt.Errorf("file output selected but file path is empty")
		}

		f, err := openLogFile(filePath)
		if err != nil {
			return nil, nil, err
		}

		cores = append(cores, zapcore.NewCore(enc, zapcore.Lock(zapcore.AddSync(f)), atomicLevel))

		return cores, []*os.File{f}, nil
	default:
	}

	return cores, nil, nil
}

func openLogFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec: G304
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	return f, nil
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
	enc.NameKey = "component"
	enc.StacktraceKey = ""

	return enc
}

// LogAuditEvent logs an audit event to the audit logger.
func LogAuditEvent(ctx context.Context, action, actor, ip, details string) {
	log := From(ctx, AuditLog)

	log.Info("Audit event",
		zap.String("action", action),
		zap.String("actor", actor),
		zap.String("ip", ip),
		zap.String("details", details),
	)

	if dbInstance == nil {
		log.Warn("cannot persist audit log: no database configured")
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Warn("failed to generate audit log id", zap.Error(err))
		return
	}

	err = dbInstance.InsertAuditLog(ctx, &dbwriter.AuditLog{
		ID:        id.String(),
		Timestamp: dbwriter.EpochMillis(time.Now()),
		Level:     "INFO",
		Actor:     actor,
		Action:    action,
		IP:        ip,
		Details:   details,
	})
	if err != nil {
		log.Warn("failed to insert audit log",
			zap.Error(err),
		)
	}
}

func RAT(val string) zap.Field           { return zap.String("rat", val) }
func Result(val string) zap.Field        { return zap.String("result", val) }
func ProcedureType(val string) zap.Field { return zap.String("type", val) }

type RegistrationOutcome struct {
	result string
	level  zapcore.Level
	failed bool
}

var (
	RegistrationAccepted     = RegistrationOutcome{result: metrics.ResultAccept, level: zapcore.InfoLevel}
	RegistrationRejected     = RegistrationOutcome{result: metrics.ResultReject, level: zapcore.InfoLevel}
	RegistrationIncompatible = RegistrationOutcome{result: metrics.ResultReject, level: zapcore.WarnLevel}
	RegistrationFailed       = RegistrationOutcome{result: metrics.ResultReject, level: zapcore.ErrorLevel, failed: true}
)

func LogRegistrationAttempt(ctx context.Context, base *zap.Logger, rat, regType string, outcome RegistrationOutcome, fields ...zap.Field) {
	metrics.RegistrationAttempt(rat, regType, outcome.result)

	msg := "UE registration accepted"
	if outcome.result != metrics.ResultAccept {
		msg = "UE registration rejected"
	}

	if outcome.failed {
		trace.SpanFromContext(ctx).SetStatus(codes.Error, msg)
	}

	From(ctx, base).WithOptions(zap.AddCallerSkip(1)).Log(outcome.level, msg,
		append([]zap.Field{RAT(rat), ProcedureType(regType), Result(outcome.result)}, fields...)...)
}

type LogDirection string

const (
	DirectionInbound  LogDirection = "inbound"
	DirectionOutbound LogDirection = "outbound"
)

type NetworkProtocol string

const (
	NGAPNetworkProtocol NetworkProtocol = "NGAP"
	S1APNetworkProtocol NetworkProtocol = "S1AP"
)

func LogNetworkEvent(
	ctx context.Context,
	protocol NetworkProtocol,
	messageType string,
	dir LogDirection,
	localAddress string,
	remoteAddress string,
	radioName string,
	rawBytes []byte,
) {
	if messageType == "" {
		From(ctx, NetworkLog).Warn("attempted to log empty network message type",
			ProtocolName(string(protocol)),
			Direction(string(dir)),
			zap.String("local_address", localAddress),
			zap.String("remote_address", remoteAddress),
		)

		return
	}

	// Count every signaling message independently of whether network event logging
	// is configured. NGAP is 5G, S1AP is 4G.
	rat := metrics.RAT5G
	if protocol == S1APNetworkProtocol {
		rat = metrics.RAT4G
	}

	metrics.SignalingMessage(rat, string(dir), messageType)

	if NetworkLog == nil {
		return
	}

	log := From(ctx, NetworkLog)

	log.Info("network_event",
		zap.String("protocol", string(protocol)),
		MessageType(messageType),
		Direction(string(dir)),
		zap.String("local_address", localAddress),
		zap.String("remote_address", remoteAddress),
		zap.String("radio_name", radioName),
	)

	if dbInstance == nil {
		log.Warn("cannot persist radio event: no database configured")
		return
	}

	err := dbInstance.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
		Timestamp:     dbwriter.EpochMillis(time.Now()),
		Protocol:      string(protocol),
		MessageType:   messageType,
		Direction:     string(dir),
		LocalAddress:  localAddress,
		RemoteAddress: remoteAddress,
		RadioName:     radioName,
		Raw:           rawBytes,
	})
	if err != nil {
		log.Warn("failed to insert radio event",
			zap.Error(err),
		)
	}
}
