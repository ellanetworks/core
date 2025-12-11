package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func handleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleRegistrationRequest")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleRegistrationRequest(ctx, ue, msg.RegistrationRequest); err != nil {
			return fmt.Errorf("failed handling registration request")
		}

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			ue.State.Set(context.Deregistered)
			if err := HandleAuthenticationError(ctx, ue); err != nil {
				return fmt.Errorf("error handling authentication error: %v", err)
			}
			return nil
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)
		}

		ue.State.Set(context.Authentication)

	case context.SecurityMode:
		ue.SecurityContextAvailable = false
		ue.T3560.Stop()
		ue.T3560 = nil
		ue.State.Set(context.Deregistered)

		return HandleGmmMessage(ctx, ue, msg)
	case context.ContextSetup:
		ue.State.Set(context.Deregistered)
		ue.GmmLog.Info("state reset to Deregistered")
		return nil
	}

	return nil
}

func handleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleServiceRequest")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleServiceRequest(ctx, ue, msg.ServiceRequest); err != nil {
			return fmt.Errorf("error handling service request: %v", err)
		}
	}

	return nil
}

func handleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleULNASTransport")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Registered:
		err := HandleULNASTransport(ctx, ue, msg.ULNASTransport)
		if err != nil {
			return fmt.Errorf("error handling UL NASTransport: %v", err)
		}
	}

	return nil
}

func handleConfigurationUpdateComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	_, span := tracer.Start(ctx, "AMF HandleConfigurationUpdateComplete")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

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
	_, span := tracer.Start(ctx, "AMF HandleNotificationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "AMF HandleDeregistrationRequestUEOriginatingDeregistration")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Registered:
		if msg == nil {
			return fmt.Errorf("gmm message is nil")
		}

		if err := HandleDeregistrationRequest(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration); err != nil {
			logger.AmfLog.Error("Error handling deregistration request", zap.Error(err))
		}

		ue.State.Set(context.DeregistrationInitiated)
	}
	return nil
}

func handleStatus5GMM(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	_, span := tracer.Start(ctx, "AMF HandleStatus5GMM")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Registered, context.Authentication, context.SecurityMode, context.ContextSetup:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}

		cause := msg.Status5GMM.Cause5GMM.GetCauseValue()
		ue.GmmLog.Error("Received Status 5GMM with cause", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
		return nil
	}
	return nil
}

func handleIdentityResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleIdentityResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	switch ue.State.Current() {
	case context.Authentication:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}

		pass, err := AuthenticationProcedure(ctx, ue)
		if err != nil {
			ue.State.Set(context.Deregistered)
			return fmt.Errorf("error in authentication procedure: %v", err)
		}
		if pass {
			ue.State.Set(context.SecurityMode)
			return securityMode(ctx, ue)
		}
		ue.State.Set(context.Authentication)
		return nil

	case context.ContextSetup:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			if err := HandleInitialRegistration(ctx, ue); err != nil {
				ue.State.Set(context.Deregistered)
				return fmt.Errorf("error handling initial registration: %v", err)
			}
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
				ue.State.Set(context.Deregistered)
				return fmt.Errorf("error handling mobility and periodic registration updating: %v", err)
			}
		}
	default:
		return fmt.Errorf("state mismatch: receive Identity Response message in state %s", ue.State.Current())
	}
	return nil
}

func handleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	ctx, span := tracer.Start(ctx, "AMF HandleAuthenticationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "AMF HandleAuthenticationFailure")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

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
		ue.State.Set(context.Deregistered)
		err := HandleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
		if err != nil {
			return fmt.Errorf("error handling security mode reject: %v", err)
		}

		ue.State.Set(context.Deregistered)

		return nil
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State.Current())
	}
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
		return handleConfigurationUpdateComplete(ctx, ue, msg)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, ue, msg)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ctx, ue, msg)
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
