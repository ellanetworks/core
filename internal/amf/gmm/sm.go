package gmm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/openapi/models"
)

func DeRegistered(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.ClearRegistrationRequestData(accessType)
		amfUe.GmmLog.Debugln("EntryEvent at GMM State[DeRegistered]")
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent at GMM State[DeRegistered]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeRegistrationRequest:
			if err := HandleRegistrationRequest(amfUe, accessType, procedureCode, gmmMessage.RegistrationRequest); err != nil {
				logger.AmfLog.Errorln(err)
			} else {
				if err := GmmFSM.SendEvent(state, StartAuthEvent, fsm.ArgsType{
					ArgAmfUe:         amfUe,
					ArgAccessType:    accessType,
					ArgProcedureCode: procedureCode,
				}); err != nil {
					logger.AmfLog.Errorln(err)
				}
			}
		case nas.MsgTypeServiceRequest:
			if err := HandleServiceRequest(amfUe, accessType, gmmMessage.ServiceRequest); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			amfUe.GmmLog.Errorf("state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case StartAuthEvent:
		logger.AmfLog.Debugln(event)
	case fsm.ExitEvent:
		logger.AmfLog.Debugln(event)
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func Registered(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		// clear stored registration request data for this registration
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.ClearRegistrationRequestData(accessType)
		amfUe.GmmLog.Debugln("EntryEvent at GMM State[Registered]")
		// store context in DB. Registration procedure is complete.
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent at GMM State[Registered]")
		switch gmmMessage.GetMessageType() {
		// Mobility Registration update / Periodic Registration update
		case nas.MsgTypeRegistrationRequest:
			if err := HandleRegistrationRequest(amfUe, accessType, procedureCode, gmmMessage.RegistrationRequest); err != nil {
				logger.AmfLog.Errorln(err)
			} else {
				if err := GmmFSM.SendEvent(state, StartAuthEvent, fsm.ArgsType{
					ArgAmfUe:         amfUe,
					ArgAccessType:    accessType,
					ArgProcedureCode: procedureCode,
				}); err != nil {
					logger.AmfLog.Errorln(err)
				}
			}
		case nas.MsgTypeULNASTransport:
			if err := HandleULNASTransport(amfUe, accessType, gmmMessage.ULNASTransport); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeConfigurationUpdateComplete:
			if err := HandleConfigurationUpdateComplete(amfUe, gmmMessage.ConfigurationUpdateComplete); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeServiceRequest:
			if err := HandleServiceRequest(amfUe, accessType, gmmMessage.ServiceRequest); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeNotificationResponse:
			if err := HandleNotificationResponse(amfUe, gmmMessage.NotificationResponse); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
			if err := GmmFSM.SendEvent(state, InitDeregistrationEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
				ArgNASMessage: gmmMessage,
			}); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeStatus5GMM:
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			amfUe.GmmLog.Errorf("state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case StartAuthEvent:
		logger.AmfLog.Debugln(event)
	case InitDeregistrationEvent:
		logger.AmfLog.Debugln(event)
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case SliceInfoAddEvent:
	case SliceInfoDeleteEvent:
	case fsm.ExitEvent:
		logger.AmfLog.Debugln(event)
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func Authentication(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	var amfUe *context.AmfUe
	switch event {
	case fsm.EntryEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		amfUe.GmmLog = amfUe.GmmLog.With(logger.FieldSuci, amfUe.Suci)
		amfUe.TxLog = amfUe.TxLog.With(logger.FieldSuci, amfUe.Suci)
		amfUe.GmmLog.Debugln("EntryEvent at GMM State[Authentication]")
		fallthrough
	case AuthRestartEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("AuthRestartEvent at GMM State[Authentication]")

		pass, err := AuthenticationProcedure(amfUe, accessType)
		if err != nil {
			if err := GmmFSM.SendEvent(state, AuthErrorEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			}); err != nil {
				logger.AmfLog.Errorln(err)
			}
		}
		if pass {
			if err := GmmFSM.SendEvent(state, AuthSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			}); err != nil {
				logger.AmfLog.Errorln(err)
			}
		}
	case GmmMessageEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent at GMM State[Authentication]")

		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeIdentityResponse:
			if err := HandleIdentityResponse(amfUe, gmmMessage.IdentityResponse); err != nil {
				logger.AmfLog.Errorln(err)
			}
			err := GmmFSM.SendEvent(state, AuthRestartEvent, fsm.ArgsType{ArgAmfUe: amfUe, ArgAccessType: accessType})
			if err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeAuthenticationResponse:
			if err := HandleAuthenticationResponse(amfUe, accessType, gmmMessage.AuthenticationResponse); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeAuthenticationFailure:
			if err := HandleAuthenticationFailure(amfUe, accessType, gmmMessage.AuthenticationFailure); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeStatus5GMM:
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			logger.AmfLog.Errorf("UE state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case AuthSuccessEvent:
		logger.AmfLog.Debugln(event)
	case AuthFailEvent:
		logger.AmfLog.Debugln(event)
		logger.AmfLog.Warnln("Reject authentication")
	case AuthErrorEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		logger.AmfLog.Debugln(event)
		if err := HandleAuthenticationError(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case fsm.ExitEvent:
		// clear authentication related data at exit
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.GmmLog.Debugln(event)
		amfUe.AuthenticationCtx = nil
		amfUe.AuthFailureCauseSynchFailureTimes = 0
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func SecurityMode(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		// set log information
		amfUe.NASLog = amfUe.NASLog.With(logger.FieldSupi, fmt.Sprintf("SUPI:%s", amfUe.Supi))
		amfUe.TxLog = amfUe.NASLog.With(logger.FieldSupi, fmt.Sprintf("SUPI:%s", amfUe.Supi))
		amfUe.GmmLog = amfUe.GmmLog.With(logger.FieldSupi, fmt.Sprintf("SUPI:%s", amfUe.Supi))
		amfUe.ProducerLog = logger.AmfLog.With(logger.FieldSupi, fmt.Sprintf("SUPI:%s", amfUe.Supi))
		amfUe.GmmLog.Debugln("EntryEvent at GMM State[SecurityMode]")
		if amfUe.SecurityContextIsValid() {
			amfUe.GmmLog.Debugln("UE has a valid security context - skip security mode control procedure")
			if err := GmmFSM.SendEvent(state, SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
				ArgNASMessage: amfUe.RegistrationRequest,
			}); err != nil {
				logger.AmfLog.Errorln(err)
			}
		} else {
			eapSuccess := args[ArgEAPSuccess].(bool)
			eapMessage := args[ArgEAPMessage].(string)
			// Select enc/int algorithm based on ue security capability & amf's policy,
			amfSelf := context.AMF_Self()
			amfUe.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)
			// Generate KnasEnc, KnasInt
			amfUe.DerivateAlgKey()
			if amfUe.CipheringAlg == security.AlgCiphering128NEA0 && amfUe.IntegrityAlg == security.AlgIntegrity128NIA0 {
				err := GmmFSM.SendEvent(state, SecuritySkipEvent, fsm.ArgsType{
					ArgAmfUe:      amfUe,
					ArgAccessType: accessType,
					ArgNASMessage: amfUe.RegistrationRequest,
				})
				if err != nil {
					logger.AmfLog.Errorln(err)
				}
			} else {
				gmm_message.SendSecurityModeCommand(amfUe.RanUe[accessType], eapSuccess, eapMessage)
			}
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent to GMM State[SecurityMode]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeSecurityModeComplete:
			if err := HandleSecurityModeComplete(amfUe, accessType, procedureCode, gmmMessage.SecurityModeComplete); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeSecurityModeReject:
			if err := HandleSecurityModeReject(amfUe, accessType, gmmMessage.SecurityModeReject); err != nil {
				logger.AmfLog.Errorln(err)
			}
			err := GmmFSM.SendEvent(state, SecurityModeFailEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeRegistrationRequest:
			// Sending AbortEvent to ongoing procedure
			err := GmmFSM.SendEvent(state, SecurityModeAbortEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Errorln(err)
			}

			err = GmmFSM.SendEvent(state, GmmMessageEvent, fsm.ArgsType{
				ArgAmfUe:         amfUe,
				ArgAccessType:    accessType,
				ArgNASMessage:    gmmMessage,
				ArgProcedureCode: procedureCode,
			})
			if err != nil {
				logger.AmfLog.Errorln(err)
			}

		case nas.MsgTypeStatus5GMM:
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			amfUe.GmmLog.Errorf("state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case SecurityModeAbortEvent:
		logger.AmfLog.Debugln(event)
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		// stopping security mode command timer
		amfUe.SecurityContextAvailable = false
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
	case NwInitiatedDeregistrationEvent:
		logger.AmfLog.Debugln(event)
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
		if err := NetworkInitiatedDeregistrationProcedure(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case SecurityModeSuccessEvent:
		logger.AmfLog.Debugln(event)
	case SecurityModeFailEvent:
		logger.AmfLog.Debugln(event)
	case fsm.ExitEvent:
		logger.AmfLog.Debugln(event)
		return
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func ContextSetup(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage]
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("EntryEvent at GMM State[ContextSetup]")
		switch message := gmmMessage.(type) {
		case *nasMessage.RegistrationRequest:
			amfUe.RegistrationRequest = message
			switch amfUe.RegistrationType5GS {
			case nasMessage.RegistrationType5GSInitialRegistration:
				if err := HandleInitialRegistration(amfUe, accessType); err != nil {
					logger.AmfLog.Errorln(err)
				}
			case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
				fallthrough
			case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
				if err := HandleMobilityAndPeriodicRegistrationUpdating(amfUe, accessType); err != nil {
					logger.AmfLog.Errorln(err)
				}
			}
		case *nasMessage.ServiceRequest:
			if err := HandleServiceRequest(amfUe, accessType, message); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			logger.AmfLog.Errorf("UE state mismatch: receieve wrong gmm message")
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent at GMM State[ContextSetup]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeIdentityResponse:
			if err := HandleIdentityResponse(amfUe, gmmMessage.IdentityResponse); err != nil {
				logger.AmfLog.Errorln(err)
			}
			switch amfUe.RegistrationType5GS {
			case nasMessage.RegistrationType5GSInitialRegistration:
				if err := HandleInitialRegistration(amfUe, accessType); err != nil {
					logger.AmfLog.Errorln(err)
					err = GmmFSM.SendEvent(state, ContextSetupFailEvent, fsm.ArgsType{
						ArgAmfUe:      amfUe,
						ArgAccessType: accessType,
					})
					if err != nil {
						logger.AmfLog.Errorln(err)
					}
				}
			case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
				fallthrough
			case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
				if err := HandleMobilityAndPeriodicRegistrationUpdating(amfUe, accessType); err != nil {
					logger.AmfLog.Errorln(err)
					err = GmmFSM.SendEvent(state, ContextSetupFailEvent, fsm.ArgsType{
						ArgAmfUe:      amfUe,
						ArgAccessType: accessType,
					})
					if err != nil {
						logger.AmfLog.Errorln(err)
					}
				}
			}
		case nas.MsgTypeRegistrationComplete:
			if err := HandleRegistrationComplete(amfUe, accessType, gmmMessage.RegistrationComplete); err != nil {
				logger.AmfLog.Errorln(err)
			}
		case nas.MsgTypeStatus5GMM:
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			amfUe.GmmLog.Errorf("state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case ContextSetupSuccessEvent:
		logger.AmfLog.Debugln(event)
	case NwInitiatedDeregistrationEvent:
		logger.AmfLog.Debugln(event)
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.T3550.Stop()
		amfUe.T3550 = nil
		amfUe.State[accessType].Set(context.Registered)
		if err := NetworkInitiatedDeregistrationProcedure(amfUe, accessType); err != nil {
			logger.AmfLog.Errorln(err)
		}
	case ContextSetupFailEvent:
		logger.AmfLog.Debugln(event)
	case fsm.ExitEvent:
		logger.AmfLog.Debugln(event)
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func DeregisteredInitiated(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if args[ArgNASMessage] != nil {
			gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
			if gmmMessage != nil {
				accessType := args[ArgAccessType].(models.AccessType)
				amfUe.GmmLog.Debugln("EntryEvent at GMM State[DeregisteredInitiated]")
				if err := HandleDeregistrationRequest(amfUe, accessType,
					gmmMessage.DeregistrationRequestUEOriginatingDeregistration); err != nil {
					logger.AmfLog.Errorln(err)
				}
			}
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debugln("GmmMessageEvent at GMM State[DeregisteredInitiated]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
			if err := HandleDeregistrationAccept(amfUe, accessType,
				gmmMessage.DeregistrationAcceptUETerminatedDeregistration); err != nil {
				logger.AmfLog.Errorln(err)
			}
		default:
			amfUe.GmmLog.Errorf("state mismatch: receieve gmm message[message type 0x%0x] at %s state",
				gmmMessage.GetMessageType(), state.Current())
		}
	case DeregistrationAcceptEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		SetDeregisteredState(amfUe, AnTypeToNas(accessType))
		logger.AmfLog.Debugln(event)
	case fsm.ExitEvent:
		logger.AmfLog.Debugln(event)
	default:
		logger.AmfLog.Errorf("Unknown event [%+v]", event)
	}
}

func SetDeregisteredState(amfUe *context.AmfUe, anType uint8) {
	amfUe.SubscriptionDataValid = false
	if anType == nasMessage.AccessType3GPP {
		amfUe.GmmLog.Warnln("UE accessType[3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType__3_GPP_ACCESS].Set(context.Deregistered)
	} else if anType == nasMessage.AccessTypeNon3GPP {
		amfUe.GmmLog.Warnln("UE accessType[Non3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType_NON_3_GPP_ACCESS].Set(context.Deregistered)
	} else {
		amfUe.GmmLog.Warnln("UE accessType[3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType__3_GPP_ACCESS].Set(context.Deregistered)
		amfUe.GmmLog.Warnln("UE accessType[Non3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType_NON_3_GPP_ACCESS].Set(context.Deregistered)
	}
}
