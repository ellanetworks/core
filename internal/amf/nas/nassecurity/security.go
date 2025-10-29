// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nassecurity

import (
	ctxt "context"
	"encoding/hex"
	"fmt"
	"reflect"
	"sync"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/security"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var mutex sync.Mutex

var tracer = otel.Tracer("ella-core/nas/security")

func Encode(ue *context.AmfUe, msg *nas.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("NAS Message is nil")
	}

	// Plain NAS message
	if ue == nil || !ue.SecurityContextAvailable {
		if msg.GmmMessage == nil {
			return nil, fmt.Errorf("msg.GmmMessage is nil")
		}
		switch msgType := msg.GmmHeader.GetMessageType(); msgType {
		case nas.MsgTypeIdentityRequest:
			if msg.GmmMessage.IdentityRequest == nil {
				return nil,
					fmt.Errorf("identity Request (type unknown) is requierd security, but security context is not available")
			}
			if identityType := msg.GmmMessage.IdentityRequest.SpareHalfOctetAndIdentityType.GetTypeOfIdentity(); identityType !=
				nasMessage.MobileIdentity5GSTypeSuci {
				return nil,
					fmt.Errorf("identity Request (%d) is requierd security, but security context is not available", identityType)
			}
		case nas.MsgTypeAuthenticationRequest:
		case nas.MsgTypeAuthenticationResult:
		case nas.MsgTypeAuthenticationReject:
		case nas.MsgTypeRegistrationReject:
		case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		case nas.MsgTypeServiceReject:
		default:
			return nil, fmt.Errorf("NAS message type %d is requierd security, but security context is not available", msgType)
		}
		pdu, err := msg.PlainNasEncode()
		return pdu, err
	} else {
		// Security protected NAS Message
		// a security protected NAS message must be integrity protected, and ciphering is optional
		needCiphering := false
		switch msg.SecurityHeader.SecurityHeaderType {
		case nas.SecurityHeaderTypeIntegrityProtected:
			ue.NASLog.Debug("Security header type: Integrity Protected")
		case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
			ue.NASLog.Debug("Security header type: Integrity Protected And Ciphered")
			needCiphering = true
		case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
			ue.NASLog.Debug("Security header type: Integrity Protected With New 5G Security Context")
			ue.ULCount.Set(0, 0)
			ue.DLCount.Set(0, 0)
		default:
			return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeader.SecurityHeaderType)
		}

		// encode plain nas first
		payload, err := msg.PlainNasEncode()
		if err != nil {
			return nil, fmt.Errorf("plain NAS encode error: %+v", err)
		}

		ue.NASLog.Debug("plain payload", zap.String("payload", hex.Dump(payload)))
		if needCiphering {
			ue.NASLog.Debug("Encrypt NAS message", zap.String("algorithm", fmt.Sprintf("%d", ue.CipheringAlg)), zap.Uint32("DLCount", ue.DLCount.Get()))
			ue.NASLog.Debug("NAS ciphering key", zap.String("key", fmt.Sprintf("%0x", ue.KnasEnc)))
			if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.DLCount.Get(),
				GetBearerType(models.AccessType3GPPAccess), security.DirectionDownlink, payload); err != nil {
				return nil, fmt.Errorf("encrypt error: %+v", err)
			}
		}

		// add sequece number
		addsqn := []byte{}
		addsqn = append(addsqn, []byte{ue.DLCount.SQN()}...)
		addsqn = append(addsqn, payload...)
		payload = addsqn

		ue.NASLog.Debug("Calculate NAS MAC", zap.String("algorithm", fmt.Sprintf("%+v", ue.IntegrityAlg)), zap.Uint32("DLCount", ue.DLCount.Get()))
		ue.NASLog.Debug("NAS integrity key", zap.String("key", fmt.Sprintf("%0x", ue.KnasInt)))
		mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.DLCount.Get(),
			GetBearerType(models.AccessType3GPPAccess), security.DirectionDownlink, payload)
		if err != nil {
			return nil, fmt.Errorf("MAC calcuate error: %+v", err)
		}
		// Add mac value
		ue.NASLog.Debug("MAC", zap.String("value", fmt.Sprintf("0x%08x", mac32)))
		addmac := []byte{}
		addmac = append(addmac, mac32...)
		addmac = append(addmac, payload...)
		payload = addmac

		// Add EPD and Security Type
		msgSecurityHeader := []byte{msg.SecurityHeader.ProtocolDiscriminator, msg.SecurityHeader.SecurityHeaderType}
		encodepayload := []byte{}
		encodepayload = append(encodepayload, msgSecurityHeader...)
		encodepayload = append(encodepayload, payload...)
		payload = encodepayload

		// Increase DL Count
		ue.DLCount.AddOne()
		return payload, nil
	}
}

func GetBearerType(accessType models.AccessType) uint8 {
	switch accessType {
	case models.AccessType3GPPAccess:
		return security.Bearer3GPP
	case models.AccessTypeNon3GPPAccess:
		return security.BearerNon3GPP
	default:
		return security.OnlyOneBearer
	}
}

func StmsiToGuti(ctx ctxt.Context, buf [7]byte) (guti string) {
	guamiList := context.GetServedGuamiList(ctx)
	servedGuami := guamiList[0]

	tmpReginID := servedGuami.AmfID[:2]
	amfID := hex.EncodeToString(buf[1:3])
	tmsi5G := hex.EncodeToString(buf[3:])

	guti = servedGuami.PlmnID.Mcc + servedGuami.PlmnID.Mnc + tmpReginID + amfID + tmsi5G

	return
}

/*
fetch Guti if present incase of integrity protected Nas Message
*/
func FetchUeContextWithMobileIdentity(ctx ctxt.Context, payload []byte) *context.AmfUe {
	if payload == nil {
		return nil
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
	logger.AmfLog.Debug("security header type", zap.Uint8("type", msg.SecurityHeaderType))
	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
		p := payload[7:]
		if err := msg.PlainNasDecode(&p); err != nil {
			return nil
		}
	case nas.SecurityHeaderTypePlainNas:
		if err := msg.PlainNasDecode(&payload); err != nil {
			return nil
		}
	default:
		logger.AmfLog.Info("Security header type is not plain or integrity protected")
		return nil
	}
	var ue *context.AmfUe = nil
	var guti string
	if msg.GmmHeader.GetMessageType() == nas.MsgTypeRegistrationRequest {
		mobileIdentity5GSContents := msg.RegistrationRequest.MobileIdentity5GS.GetMobileIdentity5GSContents()
		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			_, guti = nasConvert.GutiToString(mobileIdentity5GSContents)
			logger.AmfLog.Debug("Guti received in Registration Request Message", zap.String("guti", guti))
		} else if nasMessage.MobileIdentity5GSTypeSuci == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			suci, _ := nasConvert.SuciToString(mobileIdentity5GSContents)
			/* UeContext found based on SUCI which means context is exist in Network(AMF) but not
			   present in UE. Hence, AMF clear the existing context
			*/
			ue, _ = context.AMFSelf().AmfUeFindBySuci(suci)
			if ue != nil {
				ue.NASLog.Info("UE Context derived from Suci", zap.String("suci", suci))
				ue.SecurityContextAvailable = false
			}
			return ue
		}
	} else if msg.GmmHeader.GetMessageType() == nas.MsgTypeServiceRequest {
		mobileIdentity5GSContents := msg.ServiceRequest.TMSI5GS.Octet
		if nasMessage.MobileIdentity5GSType5gSTmsi == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			guti = StmsiToGuti(ctx, mobileIdentity5GSContents)
			logger.AmfLog.Debug("Guti derived from Service Request Message", zap.String("guti", guti))
		}
	} else if msg.GmmHeader.GetMessageType() == nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration {
		mobileIdentity5GSContents := msg.DeregistrationRequestUEOriginatingDeregistration.MobileIdentity5GS.GetMobileIdentity5GSContents()
		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			_, guti = nasConvert.GutiToString(mobileIdentity5GSContents)
			logger.AmfLog.Debug("Guti received in Deregistraion Request Message", zap.String("guti", guti))
		}
	}
	if guti != "" {
		ue, _ = context.AMFSelf().AmfUeFindByGuti(guti)
		if ue != nil {
			if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
				ue.NASLog.Info("UE Context derived from Guti but received in plain nas", zap.String("guti", guti))
				return nil
			}
			ue.NASLog.Info("UE Context derived from Guti", zap.String("guti", guti))
			return ue
		} else {
			logger.AmfLog.Warn("UE Context not fround", zap.String("guti", guti))
		}
	}

	return nil
}

/*
payload either a security protected 5GS NAS message or a plain 5GS NAS message which
format is followed TS 24.501 9.1.1
*/
func Decode(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, payload []byte) (*nas.Message, error) {
	_, span := tracer.Start(ctx, "AMF NAS Decode",
		trace.WithAttributes(
			attribute.String("nas.accessType", string(accessType)),
		),
	)
	defer span.End()
	if ue == nil {
		return nil, fmt.Errorf("amf ue is nil")
	}
	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	var integrityProtected bool
	ulCountNew := ue.ULCount

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		// RRCEstablishmentCause 0 is for emergency service
		if ue.SecurityContextAvailable && ue.RanUe[accessType].RRCEstablishmentCause != "0" {
			ue.NASLog.Debug("Received Plain NAS message")
			ue.MacFailed = false
			ue.SecurityContextAvailable = false
			if err := msg.PlainNasDecode(&payload); err != nil {
				return nil, err
			}

			if msg.GmmMessage == nil {
				return nil, fmt.Errorf("gmm message is nil")
			}

			// TS 24.501 4.4.4.3: Except the messages listed below, no NAS signalling messages shall be processed
			// by the receiving 5GMM entity in the AMF or forwarded to the 5GSM entity, unless the secure exchange
			// of NAS messages has been established for the NAS signalling connection
			switch msg.GmmHeader.GetMessageType() {
			case nas.MsgTypeRegistrationRequest:
				return msg, nil
			case nas.MsgTypeIdentityResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationFailure:
				return msg, nil
			case nas.MsgTypeSecurityModeReject:
				return msg, nil
			case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
				return msg, nil
			case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
				return msg, nil
			default:
				return nil, fmt.Errorf(
					"UE can not send plain nas for non-emergency service when there is a valid security context")
			}
		} else {
			ue.MacFailed = false
			err := msg.PlainNasDecode(&payload)
			return msg, err
		}
	} else { // Security protected NAS message
		if len(payload) < 7 {
			return nil, fmt.Errorf("nas payload is too short")
		}
		securityHeader := payload[0:6]
		sequenceNumber := payload[6]

		receivedMac32 := securityHeader[2:]
		// remove security Header except for sequece Number
		payload = payload[6:]

		// a security protected NAS message must be integrity protected, and ciphering is optional
		ciphered := false
		switch msg.SecurityHeaderType {
		case nas.SecurityHeaderTypeIntegrityProtected:
			ue.NASLog.Debug("Security header type: Integrity Protected")
		case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
			ue.NASLog.Debug("Security header type: Integrity Protected And Ciphered")
			ciphered = true
		case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
			ue.NASLog.Debug("Security header type: Integrity Protected And Ciphered With New 5G Security Context")
			ciphered = true
			ulCountNew.Set(0, 0)
		default:
			return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeader.SecurityHeaderType)
		}

		if ciphered && !ue.SecurityContextAvailable {
			return nil, fmt.Errorf("NAS message is ciphered, but UE Security Context is not Available")
		}

		if ue.SecurityContextAvailable {
			if ulCountNew.SQN() > sequenceNumber {
				ue.NASLog.Debug("set ULCount overflow")
				ulCountNew.SetOverflow(ulCountNew.Overflow() + 1)
			}
			ulCountNew.SetSQN(sequenceNumber)

			ue.NASLog.Debug("Calculate NAS MAC", zap.Uint8("algorithm", ue.IntegrityAlg), zap.Uint32("ULCount", ulCountNew.Get()))
			ue.NASLog.Debug("NAS integrity key", zap.String("key", fmt.Sprintf("%0x", ue.KnasInt)))
			var mac32 []byte
			mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ulCountNew.Get(),
				GetBearerType(accessType), security.DirectionUplink, payload)
			if err != nil {
				return nil, fmt.Errorf("MAC calcuate error: %+v", err)
			}

			if !reflect.DeepEqual(mac32, receivedMac32) {
				ue.NASLog.Warn("NAS MAC verification failed", zap.String("received", hex.EncodeToString(receivedMac32)), zap.String("expected", hex.EncodeToString(mac32)))
			} else {
				ue.NASLog.Debug("cmac value", zap.String("cmac", fmt.Sprintf("%0x", mac32)))
				integrityProtected = true
			}
		} else {
			ue.NASLog.Debug("UE Security Context is not Available, so skip MAC verify")
		}

		if ciphered {
			if !integrityProtected {
				return nil, fmt.Errorf("NAS message is ciphered, but MAC verification failed")
			}
			ue.NASLog.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("ULCount", ulCountNew.Get()))
			ue.NASLog.Debug("NAS ciphering key", zap.String("key", fmt.Sprintf("%0x", ue.KnasEnc)))
			// decrypt payload without sequence number (payload[1])
			if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ulCountNew.Get(), GetBearerType(accessType),
				security.DirectionUplink, payload[1:]); err != nil {
				return nil, fmt.Errorf("decrypt error: %+v", err)
			}
		}

		// if ue.ULCount.SQN() > sequenceNumber {
		// 	ue.NASLog.Debug("set ULCount overflow")
		// 	ue.ULCount.SetOverflow(ue.ULCount.Overflow() + 1)
		// }
		// ue.ULCount.SetSQN(sequenceNumber)

		// mutex.Lock()
		// defer mutex.Unlock()
		// mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.ULCount.Get(), security.Bearer3GPP,
		// 	security.DirectionUplink, payload)
		// if err != nil {
		// 	return nil, fmt.Errorf("error calculating mac: %+v", err)
		// }

		// if !reflect.DeepEqual(mac32, receivedMac32) {
		// 	ue.NASLog.Warn("MAC verification failed", zap.String("received", hex.EncodeToString(receivedMac32)), zap.String("expected", hex.EncodeToString(mac32)))
		// 	ue.MacFailed = true
		// } else {
		// 	ue.MacFailed = false
		// 	integrityProtected = true
		// }

		// if ciphered {
		// 	ue.NASLog.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("ULCount", ue.ULCount.Get()))
		// 	// decrypt payload without sequence number (payload[1])
		// 	if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP,
		// 		security.DirectionUplink, payload[1:]); err != nil {
		// 		return nil, fmt.Errorf("error encrypting: %+v", err)
		// 	}
		// }

		// remove sequece Number
		payload = payload[1:]
		err := msg.PlainNasDecode(&payload)

		/*
			integrity check failed, as per spec 24501 section 4.4.4.3 AMF shouldnt process or forward to SMF
			except below message types
		*/
		if err == nil && ue.MacFailed {
			switch msg.GmmHeader.GetMessageType() {
			case nas.MsgTypeRegistrationRequest:
				return msg, nil
			case nas.MsgTypeIdentityResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationFailure:
				return msg, nil
			case nas.MsgTypeSecurityModeReject:
				return msg, nil
			case nas.MsgTypeServiceRequest:
				return msg, nil
			case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
				return msg, nil
			case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
				return msg, nil
			default:
				return nil, fmt.Errorf("mac verification failed for the nas message: %v", msg.GmmHeader.GetMessageType())
			}
		}

		if integrityProtected {
			ue.ULCount = ulCountNew
		}

		logger.AmfLog.Warn("TO DELETE: UL Count after NAS Decode", zap.Uint32("ULCount", ue.ULCount.Get()))

		return msg, err
	}
}
