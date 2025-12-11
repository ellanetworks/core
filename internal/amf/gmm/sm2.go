package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func handleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleRegistrationRequest(ctx, ue, msg.RegistrationRequest); err != nil {
			return fmt.Errorf("failed handling registration request")
		}

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			if err := GmmFSM.SendEvent(ctx, ue.State, AuthErrorEvent, fsm.ArgsType{
				ArgAmfUe: ue,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)

		}

		ue.State.Set(context.Authentication)

	case context.SecurityMode:
		err := GmmFSM.SendEvent(ctx, ue.State, SecurityModeAbortEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}

		return HandleGmmMessage(ctx, ue, msg)
	case context.ContextSetup:
		err := GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}

		ue.GmmLog.Info("state reset to Deregistered")
	}

	return nil
}

func handleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleServiceRequest(ctx, ue, msg.ServiceRequest); err != nil {
			return fmt.Errorf("error handling service request: %v", err)
		}
	}

	return nil
}

func handleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleULNASTransport(ctx, ue, msg.ULNASTransport)
		if err != nil {
			return fmt.Errorf("error handling UL NASTransport: %v", err)
		}
	}

	return nil
}

func handleConfigurationUpdateComplete(ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleConfigurationUpdateComplete(ue, msg.ConfigurationUpdateComplete)
		if err != nil {
			return fmt.Errorf("error handling configuration update complete: %v", err)
		}
	}
	return nil
}

func handleNotificationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleNotificationResponse(ctx, ue, msg.NotificationResponse)
		if err != nil {
			return fmt.Errorf("error handling notification response: %v", err)
		}
	}
	return nil
}

func handleDeregistrationRequestUEOriginatingDeregistration(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := GmmFSM.SendEvent(ctx, ue.State, InitDeregistrationEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgNASMessage: msg,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	}
	return nil
}

func handleStatus5GMM(ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered, context.Authentication, context.SecurityMode, context.ContextSetup:
		if err := HandleStatus5GMM(ue, msg.Status5GMM); err != nil {
			return fmt.Errorf("error handling status 5GMM: %v", err)
		}
	}
	return nil
}

func handleIdentityResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		ue.GmmLog.Debug("AuthRestartEvent at GMM State[Authentication]")

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			if err := GmmFSM.SendEvent(ctx, ue.State, AuthErrorEvent, fsm.ArgsType{
				ArgAmfUe: ue,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)
		}
		ue.State.Set(context.Authentication)

	case context.ContextSetup:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			if err := HandleInitialRegistration(ctx, ue); err != nil {
				logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
				err = GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
					ArgAmfUe: ue,
				})
				if err != nil {
					return fmt.Errorf("error sending event: %v", err)
				}
			}
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
				logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
				err = GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
					ArgAmfUe: ue,
				})
				if err != nil {
					return fmt.Errorf("error sending event: %v", err)
				}
			}
		}
	default:
		return fmt.Errorf("state mismatch: receive Identity Response message in state %s", ue.State.Current())
	}
	return nil
}

func handleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationResponse(ctx, ue, msg.AuthenticationResponse)
		if err != nil {
			return fmt.Errorf("error handling authentication response: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Response message in state %s", ue.State.Current())
	}
	return nil
}

func handleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationFailure(ctx, ue, msg.AuthenticationFailure)
		if err != nil {
			return fmt.Errorf("error handling authentication failure :%v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Failure message in state %s", ue.State.Current())
	}

	return nil
}

func handleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.SecurityMode:
		err := HandleSecurityModeComplete(ctx, ue, msg.SecurityModeComplete)
		if err != nil {
			return fmt.Errorf("error handling security mode complete: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", ue.State.Current())
	}

	return nil
}

func handleSecurityModeReject(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.SecurityMode:
		err := HandleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
		if err != nil {
			return fmt.Errorf("error handling security mode reject: %v", err)
		}

		err = GmmFSM.SendEvent(ctx, ue.State, SecurityModeFailEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State.Current())
	}
	return nil
}

func handleRegistrationComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.ContextSetup:
		if err := HandleRegistrationComplete(ctx, ue, msg.RegistrationComplete); err != nil {
			return fmt.Errorf("error handling registration complete: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.State.Current())
	}

	return nil
}

func handleDeregistrationAccept(ctx ctxt.Context, ue *context.AmfUe, msg *nasMessage.DeregistrationAcceptUETerminatedDeregistration) error {
	switch ue.State.Current() {
	case context.DeregistrationInitiated:
		if err := HandleDeregistrationAccept(ctx, ue, msg); err != nil {
			return fmt.Errorf("error handling deregistration accept: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Deregistration Accept message in state %s", ue.State.Current())
	}

	return nil
}

func HandleGmmMessage(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	msgType := msg.GetMessageType()
	ctx, span := tracer.Start(ctx, "AMF HandleGmmMessage")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
		attribute.String("messageType", getMessageName(msgType)),
	)
	defer span.End()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, ue, msg)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, ue, msg)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, ue, msg)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(ue, msg)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, ue, msg)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, ue, msg)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, ue, msg)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, ue, msg)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue, msg.DeregistrationAcceptUETerminatedDeregistration)
	default:
		return fmt.Errorf("message type %d handling not implemented", msg.GetMessageType())
	}
}

func getMessageName(msgType uint8) string {
	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return "RegistrationRequest"
	case nas.MsgTypeRegistrationAccept:
		return "RegistrationAccept"
	case nas.MsgTypeRegistrationComplete:
		return "RegistrationComplete"
	case nas.MsgTypeRegistrationReject:
		return "RegistrationReject"
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return "DeregistrationRequestUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return "DeregistrationAcceptUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return "DeregistrationRequestUETerminatedDeregistration"
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return "DeregistrationAcceptUETerminatedDeregistration"
	case nas.MsgTypeServiceRequest:
		return "ServiceRequest"
	case nas.MsgTypeServiceReject:
		return "ServiceReject"
	case nas.MsgTypeServiceAccept:
		return "ServiceAccept"
	case nas.MsgTypeConfigurationUpdateCommand:
		return "ConfigurationUpdateCommand"
	case nas.MsgTypeConfigurationUpdateComplete:
		return "ConfigurationUpdateComplete"
	case nas.MsgTypeAuthenticationRequest:
		return "AuthenticationRequest"
	case nas.MsgTypeAuthenticationResponse:
		return "AuthenticationResponse"
	case nas.MsgTypeAuthenticationReject:
		return "AuthenticationReject"
	case nas.MsgTypeAuthenticationFailure:
		return "AuthenticationFailure"
	case nas.MsgTypeAuthenticationResult:
		return "AuthenticationResult"
	case nas.MsgTypeIdentityRequest:
		return "IdentityRequest"
	case nas.MsgTypeIdentityResponse:
		return "IdentityResponse"
	case nas.MsgTypeSecurityModeCommand:
		return "SecurityModeCommand"
	case nas.MsgTypeSecurityModeComplete:
		return "SecurityModeComplete"
	case nas.MsgTypeSecurityModeReject:
		return "SecurityModeReject"
	case nas.MsgTypeStatus5GMM:
		return "Status5GMM"
	case nas.MsgTypeNotification:
		return "Notification"
	case nas.MsgTypeNotificationResponse:
		return "NotificationResponse"
	case nas.MsgTypeULNASTransport:
		return "ULNASTransport"
	case nas.MsgTypeDLNASTransport:
		return "DLNASTransport"
	default:
		return "Unknown"
	}
}
