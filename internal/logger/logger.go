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

	auditDBSink   zapcore.WriteSyncer
	networkDBSink zapcore.WriteSyncer
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

	networkCores, err := makeCores(systemOutput, systemFilePath, consoleEnc, jsonEnc)
	if err != nil {
		return fmt.Errorf("could not make cores: %w", err)
	}

	if networkDBSink != nil {
		networkCores = append(networkCores, zapcore.NewCore(jsonEnc, networkDBSink, atomicLevel))
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

func SetNetworkDBWriter(writeFn func([]byte) error) {
	if writeFn == nil {
		networkDBSink = nil
		return
	}
	ws := zapcore.AddSync(funcWriteSyncer{write: writeFn})
	networkDBSink = ws

	// If the network logger already exists, attach the DB core now.
	if NetworkLog != nil {
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(prodJSONEncoderConfig()),
			networkDBSink,
			atomicLevel,
		)
		NetworkLog = NetworkLog.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
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

type NetworkMessageType string

const (
	// Access events (inbound)
	SubscriberRegistrationRequest                     NetworkMessageType = "Registration Request"
	SubscriberInitialRegistration                     NetworkMessageType = "Initial Registration"
	SubscriberMobilityAndPeriodicRegistrationUpdating NetworkMessageType = "Mobility and Periodic Registration Updating"
	SubscriberIdentityResponse                        NetworkMessageType = "Identity Response"
	SubscriberNotificationResponse                    NetworkMessageType = "Notification Response"
	SubscriberConfigurationUpdateComplete             NetworkMessageType = "Configuration Update Complete"
	SubscriberServiceRequest                          NetworkMessageType = "Service Request"
	SubscriberAuthenticationResponse                  NetworkMessageType = "Authentication Response"
	SubscriberAuthenticationFailure                   NetworkMessageType = "Authentication Failure"
	SubscriberRegistrationComplete                    NetworkMessageType = "Registration Complete"
	SubscriberSecurityModeComplete                    NetworkMessageType = "Security Mode Complete"
	SubscriberSecurityModeReject                      NetworkMessageType = "Security Mode Reject"
	SubscriberDeregistrationRequest                   NetworkMessageType = "Deregistration Request"
	SubscriberDeregistrationAccept                    NetworkMessageType = "Deregistration Accept"
	SubscriberStatus5GMM                              NetworkMessageType = "Status 5GMM"

	// Access events (outbound)
	SubscriberRegistrationAccept    NetworkMessageType = "Registration Accept"
	SubscriberSecurityModeCommand   NetworkMessageType = "Security Mode Command"
	SubscriberRegistrationReject    NetworkMessageType = "Registration Reject"
	SubscriberIdentityRequest       NetworkMessageType = "Identity Request"
	SubscriberNotification          NetworkMessageType = "Notification"
	SubscriberAuthenticationRequest NetworkMessageType = "Authentication Request"
	SubscriberAuthenticationResult  NetworkMessageType = "Authentication Result"
	SubscriberAuthenticationReject  NetworkMessageType = "Authentication Reject"
	SubscriberServiceAccept         NetworkMessageType = "Service Accept"
	SubscriberServiceReject         NetworkMessageType = "Service Reject"

	// Session events (inbound)
	SubscriberPduSessionEstablishmentRequest NetworkMessageType = "PDU Session Establishment Request"

	// Session events (outbound)
	SubscriberPduSessionEstablishmentReject NetworkMessageType = "PDU Session Establishment Reject"
	SubscriberPduSessionEstablishmentAccept NetworkMessageType = "PDU Session Establishment Accept"
)

const (
	// Radio events (inbound)
	RadioNGSetupRequest                      NetworkMessageType = "NG Setup Request"
	RadioUplinkNASTransport                  NetworkMessageType = "Uplink NAS Transport"
	RadioNGReset                             NetworkMessageType = "NG Reset"
	RadioNGResetAcknowledge                  NetworkMessageType = "NG Reset Acknowledge"
	RadioUEContextReleaseComplete            NetworkMessageType = "UE Context Release Complete"
	RadioPDUSessionResourceReleaseResponse   NetworkMessageType = "PDU Session Resource Release Response"
	RadioUERadioCapabilityCheckResponse      NetworkMessageType = "UE Radio Capability Check Response"
	RadioLocationReportingFailureIndication  NetworkMessageType = "Location Reporting Failure Indication"
	RadioInitialUEMessage                    NetworkMessageType = "Initial UE Message"
	RadioPDUSessionResourceSetupResponse     NetworkMessageType = "PDU Session Resource Setup Response"
	RadioPDUSessionResourceModifyResponse    NetworkMessageType = "PDU Session Resource Modify Response"
	RadioPDUSessionResourceNotify            NetworkMessageType = "PDU Session Resource Notify"
	RadioPDUSessionResourceModifyIndication  NetworkMessageType = "PDU Session Resource Modify Indication"
	RadioInitialContextSetupResponse         NetworkMessageType = "Initial Context Setup Response"
	RadioInitialContextSetupFailure          NetworkMessageType = "Initial Context Setup Failure"
	RadioUEContextReleaseRequest             NetworkMessageType = "UE Context Release Request"
	RadioUEContextModificationResponse       NetworkMessageType = "UE Context Modification Response"
	RadioUEContextModificationFailure        NetworkMessageType = "UE Context Modification Failure"
	RadioRRCInactiveTransitionReport         NetworkMessageType = "RRC Inactive Transition Report"
	RadioHandoverNotify                      NetworkMessageType = "Handover Notify"
	RadioPathSwitchRequest                   NetworkMessageType = "Path Switch Request"
	RadioHandoverRequestAcknowledge          NetworkMessageType = "Handover Request Acknowledge"
	RadioHandoverFailure                     NetworkMessageType = "Handover Failure"
	RadioHandoverRequired                    NetworkMessageType = "Handover Required"
	RadioHandoverCancel                      NetworkMessageType = "Handover Cancel"
	RadioUplinkRanStatusTransfer             NetworkMessageType = "Uplink RAN Status Transfer"
	RadioNasNonDeliveryIndication            NetworkMessageType = "NAS Non Delivery Indication"
	RadioRanConfigurationUpdate              NetworkMessageType = "RAN Configuration Update"
	RadioUplinkRanConfigurationTransfer      NetworkMessageType = "Uplink RAN Configuration Transfer"
	RadioUplinkUEAssociatedNRPPATransport    NetworkMessageType = "Uplink UE Associated NRPPA Transport"
	RadioUplinkNonUEAssociatedNRPPATransport NetworkMessageType = "Uplink Non UE Associated NRPPA Transport"
	RadioLocationReport                      NetworkMessageType = "Location Report"
	RadioUERadioCapabilityInfoIndication     NetworkMessageType = "UE Radio Capability Info Indication"
	RadioAMFConfigurationUpdateFailure       NetworkMessageType = "AMF Configuration Update Failure"
	RadioAMFConfigurationUpdateAcknowledge   NetworkMessageType = "AMF Configuration Update Acknowledge"
	RadioErrorIndication                     NetworkMessageType = "Error Indication"
	RadioCellTrafficTrace                    NetworkMessageType = "Cell Traffic Trace"

	// Radio events (outbound)
	RadioNGSetupResponse                   NetworkMessageType = "NG Setup Response"
	RadioNGSetupFailure                    NetworkMessageType = "NG Setup Failure"
	RadioDownlinkNasTransport              NetworkMessageType = "Downlink NAS Transport"
	RadioPDUSessionResourceReleaseCommand  NetworkMessageType = "PDU Session Resource Release Command"
	RadioUEContextReleaseCommand           NetworkMessageType = "UE Context Release Command"
	RadioPDUSessionResourceSetupRequest    NetworkMessageType = "PDU Session Resource Setup Request"
	RadioPDUSessionResourceModifyConfirm   NetworkMessageType = "PDU Session Resource Modify Confirm"
	RadioPDUSessionResourceModifyRequest   NetworkMessageType = "PDU Session Resource Modify Request"
	RadioInitialContextSetupRequest        NetworkMessageType = "Initial Context Setup Request"
	RadioHandoverCommand                   NetworkMessageType = "Handover Command"
	RadioHandoverPreparationFailure        NetworkMessageType = "Handover Preparation Failure"
	RadioHandoverRequest                   NetworkMessageType = "Handover Request"
	RadioPathSwitchRequestAcknowledge      NetworkMessageType = "Path Switch Request Acknowledge"
	RadioPathSwitchRequestFailure          NetworkMessageType = "Path Switch Request Failure"
	RadioRanConfigurationUpdateAcknowledge NetworkMessageType = "RAN Configuration Update Acknowledge"
	RadioRanConfigurationUpdateFailure     NetworkMessageType = "RAN Configuration Update Failure"
	RadioAMFStatusIndication               NetworkMessageType = "AMF Status Indication"
	RadioDownlinkRanConfigurationTransfer  NetworkMessageType = "Downlink RAN Configuration Transfer"
	RadioLocationReportingControl          NetworkMessageType = "Location Reporting Control"
)

type LogDirection string

const (
	DirectionInbound  LogDirection = "inbound"
	DirectionOutbound LogDirection = "outbound"
)

type NetworkProtocol string

const (
	NGAPNetworkProtocol NetworkProtocol = "NGAP"
	NASNetworkProtocol  NetworkProtocol = "NAS"
)

func LogNetworkEvent(protocol NetworkProtocol, messageType NetworkMessageType, dir LogDirection, rawBytes []byte, fields ...zap.Field) {
	if NetworkLog == nil {
		return
	}

	if messageType == "" {
		EllaLog.Warn("attempted to log empty network message type",
			zap.String("protocol", string(protocol)),
			zap.String("dir", string(dir)),
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
		"message_type": {},
		"direction":    {},
		"raw":          {},
		"protocol":     {},
		"timestamp":    {},
		"component":    {},
		"caller":       {},
		"message":      {},
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
	NetworkLog.Info("network_event",
		zap.String("protocol", string(protocol)),
		zap.String("message_type", string(messageType)),
		zap.String("direction", string(dir)),
		zap.Binary("raw", rawBytes),
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
