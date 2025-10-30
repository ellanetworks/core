// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nassecurity

import (
	ctxt "context"
	"encoding/binary"
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
	"go.uber.org/zap"
)

var mutex sync.Mutex

var tracer = otel.Tracer("ella-core/nas/security")

func Encode(ue *context.AmfUe, msg *nas.Message) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("amf ue is nil")
	}
	if msg == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	// Plain NAS message
	if !ue.SecurityContextAvailable {
		return msg.PlainNasEncode()
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
			return nil, fmt.Errorf("error encoding plain nas: %+v", err)
		}

		if needCiphering {
			ue.NASLog.Debug("Encrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("DLCount", ue.DLCount.Get()))
			if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.DLCount.Get(), security.Bearer3GPP,
				security.DirectionDownlink, payload); err != nil {
				return nil, fmt.Errorf("error encrypting: %+v", err)
			}
		}

		// add sequece number
		payload = append([]byte{ue.DLCount.SQN()}, payload[:]...)

		ue.NASLog.Debug("Calculate NAS MAC", zap.Uint8("algorithm", ue.IntegrityAlg), zap.Uint32("DLCount", ue.DLCount.Get()))
		ue.NASLog.Debug("NAS integrity key", zap.Any("key", ue.KnasInt))
		mutex.Lock()
		defer mutex.Unlock()
		mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.DLCount.Get(), security.Bearer3GPP,
			security.DirectionDownlink, payload)
		if err != nil {
			return nil, fmt.Errorf("MAC calcuate error: %+v", err)
		}
		// Add mac value
		payload = append(mac32, payload[:]...)

		// Add EPD and Security Type
		msgSecurityHeader := []byte{msg.SecurityHeader.ProtocolDiscriminator, msg.SecurityHeader.SecurityHeaderType}
		payload = append(msgSecurityHeader, payload[:]...)

		// Increase DL Count
		ue.DLCount.AddOne()
		return payload, nil
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
				logger.AmfLog.Warn("TO DELETE: Security context marked as unavailable based on SUCI", zap.String("supi", ue.Supi))
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
			logger.AmfLog.Warn("UE Context not found", zap.String("guti", guti))
		}
	}

	return nil
}

/*
payload either a security protected 5GS NAS message or a plain 5GS NAS message which
format is followed TS 24.501 9.1.1
*/
// func Decode(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, payload []byte) (*nas.Message, error) {
// 	_, span := tracer.Start(ctx, "AMF NAS Decode",
// 		trace.WithAttributes(
// 			attribute.String("nas.accessType", string(accessType)),
// 		),
// 	)
// 	defer span.End()
// 	if ue == nil {
// 		return nil, fmt.Errorf("amf ue is nil")
// 	}
// 	if payload == nil {
// 		return nil, fmt.Errorf("nas payload is empty")
// 	}

// 	msg := new(nas.Message)
// 	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
// 	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
// 		// RRCEstablishmentCause 0 is for emergency service
// 		if ue.SecurityContextAvailable && ue.RanUe[accessType].RRCEstablishmentCause != "0" {
// 			ue.NASLog.Warn("Received Plain NAS message")
// 			ue.MacFailed = false
// 			// ue.SecurityContextAvailable = false
// 			if err := msg.PlainNasDecode(&payload); err != nil {
// 				return nil, err
// 			}

// 			if msg.GmmMessage == nil {
// 				return nil, fmt.Errorf("gmm message is nil")
// 			}

// 			// TS 24.501 4.4.4.3: Except the messages listed below, no NAS signalling messages shall be processed
// 			// by the receiving 5GMM entity in the AMF or forwarded to the 5GSM entity, unless the secure exchange
// 			// of NAS messages has been established for the NAS signalling connection
// 			switch msg.GmmHeader.GetMessageType() {
// 			case nas.MsgTypeRegistrationRequest:
// 				return msg, nil
// 			case nas.MsgTypeIdentityResponse:
// 				return msg, nil
// 			case nas.MsgTypeAuthenticationResponse:
// 				return msg, nil
// 			case nas.MsgTypeAuthenticationFailure:
// 				return msg, nil
// 			case nas.MsgTypeSecurityModeReject:
// 				return msg, nil
// 			case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
// 				return msg, nil
// 			case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
// 				return msg, nil
// 			default:
// 				return nil, fmt.Errorf(
// 					"UE can not send plain nas for non-emergency service when there is a valid security context")
// 			}
// 		} else {
// 			ue.MacFailed = false
// 			err := msg.PlainNasDecode(&payload)
// 			return msg, err
// 		}
// 	} else { // Security protected NAS message
// 		if len(payload) < 7 {
// 			return nil, fmt.Errorf("nas payload is too short")
// 		}
// 		securityHeader := payload[0:6]
// 		sequenceNumber := payload[6]

// 		receivedMac32 := securityHeader[2:]
// 		// remove security Header except for sequece Number
// 		payload = payload[6:]

// 		// a security protected NAS message must be integrity protected, and ciphering is optional
// 		ciphered := false
// 		switch msg.SecurityHeaderType {
// 		case nas.SecurityHeaderTypeIntegrityProtected:
// 			ue.NASLog.Debug("Security header type: Integrity Protected")
// 		case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
// 			ue.NASLog.Debug("Security header type: Integrity Protected And Ciphered")
// 			ciphered = true
// 		case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
// 			ue.NASLog.Debug("Security header type: Integrity Protected And Ciphered With New 5G Security Context")
// 			ciphered = true
// 			ue.ULCount.Set(0, 0)
// 		default:
// 			return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeader.SecurityHeaderType)
// 		}

// 		if ue.SecurityContextAvailable {
// 			if ulCountNew.SQN() > sequenceNumber {
// 				ue.NASLog.Debugf("set ULCount overflow")
// 				ulCountNew.SetOverflow(ulCountNew.Overflow() + 1)
// 			}
// 			ulCountNew.SetSQN(sequenceNumber)

// 			if ue.ULCount.SQN() > sequenceNumber {
// 				ue.NASLog.Debug("set ULCount overflow")
// 				ue.ULCount.SetOverflow(ue.ULCount.Overflow() + 1)
// 			}
// 			ue.ULCount.SetSQN(sequenceNumber)

// 			mutex.Lock()
// 			defer mutex.Unlock()
// 			mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.ULCount.Get(), security.Bearer3GPP,
// 				security.DirectionUplink, payload)
// 			if err != nil {
// 				return nil, fmt.Errorf("error calculating mac: %+v", err)
// 			}

// 			if !reflect.DeepEqual(mac32, receivedMac32) {
// 				ue.NASLog.Warn("MAC verification failed", zap.String("received", hex.EncodeToString(receivedMac32)), zap.String("expected", hex.EncodeToString(mac32)))
// 				ue.MacFailed = true
// 			} else {
// 				ue.MacFailed = false
// 			}

// 			// if ciphered {
// 			// 	ue.NASLog.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("ULCount", ue.ULCount.Get()))
// 			// 	// decrypt payload without sequence number (payload[1])
// 			// 	if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP,
// 			// 		security.DirectionUplink, payload[1:]); err != nil {
// 			// 		return nil, fmt.Errorf("error encrypting: %+v", err)
// 			// 	}
// 			// }

// 			// remove sequece Number
// 			payload = payload[1:]
// 		}

// 		if ciphered {
// 			if !integrityProtected {
// 				return nil, false, fmt.Errorf("NAS message is ciphered, but MAC verification failed")
// 			}
// 			ue.NASLog.Debugf("Decrypt NAS message (algorithm: %+v, ULCount: 0x%0x)", ue.CipheringAlg, ulCountNew.Get())
// 			ue.NASLog.Tracef("NAS ciphering key: %0x", ue.KnasEnc)
// 			// decrypt payload without sequence number (payload[1])
// 			if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ulCountNew.Get(), GetBearerType(accessType),
// 				security.DirectionUplink, payload[1:]); err != nil {
// 				return nil, false, fmt.Errorf("decrypt error: %+v", err)
// 			}
// 		}

// 		// remove sequece Number
// 		payload = payload[1:]
// 	}

// 	err = msg.PlainNasDecode(&payload)

// 	/*
// 		integrity check failed, as per spec 24501 section 4.4.4.3 AMF shouldnt process or forward to SMF
// 		except below message types
// 	*/
// 	if err == nil && ue.MacFailed {
// 		switch msg.GmmHeader.GetMessageType() {
// 		case nas.MsgTypeRegistrationRequest:
// 			return msg, nil
// 		case nas.MsgTypeIdentityResponse:
// 			return msg, nil
// 		case nas.MsgTypeAuthenticationResponse:
// 			return msg, nil
// 		case nas.MsgTypeAuthenticationFailure:
// 			return msg, nil
// 		case nas.MsgTypeSecurityModeReject:
// 			return msg, nil
// 		case nas.MsgTypeServiceRequest:
// 			return msg, nil
// 		case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
// 			return msg, nil
// 		case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
// 			return msg, nil
// 		default:
// 			return nil, fmt.Errorf("mac verification failed for the nas message: %v", msg.GmmHeader.GetMessageType())
// 		}
// 	}

// 	return msg, err

// }

// ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, payload []byte) (*nas.Message, error
func Decode(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, payload []byte, initialMessage bool,
) (msg *nas.Message, err error) {
	var integrityProtected bool
	if ue == nil {
		return nil, fmt.Errorf("amfUe is nil")
	}
	if payload == nil {
		return nil, fmt.Errorf("NAS payload is empty")
	}
	if len(payload) < 2 {
		return nil, fmt.Errorf("NAS payload is too short")
	}

	ulCountNew := ue.ULCount

	msg = new(nas.Message)
	msg.ProtocolDiscriminator = payload[0]
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
	// ue.NASLog.Traceln("securityHeaderType is ", msg.SecurityHeaderType)
	if msg.SecurityHeaderType != nas.SecurityHeaderTypePlainNas { // Security protected NAS message
		// Extended protocol discriminator	V 1
		// Security header type				V 1/2
		// Spare half octet					V 1/2
		// Message authentication code		V 4
		// Sequence number					V 1
		// Plain 5GS NAS message			V 3-n
		if len(payload) < (1 + 1 + 4 + 1 + 3) {
			return nil, fmt.Errorf("NAS payload is too short")
		}
		securityHeader := payload[0:6]
		// ue.NASLog.Traceln("securityHeader is ", securityHeader)
		sequenceNumber := payload[6]
		// ue.NASLog.Traceln("sequenceNumber", sequenceNumber)
		msg.SequenceNumber = sequenceNumber
		receivedMac32 := securityHeader[2:]
		msg.MessageAuthenticationCode = binary.BigEndian.Uint32(receivedMac32)
		// remove security Header except for sequece Number
		payload = payload[6:]

		// a security protected NAS message must be integrity protected, and ciphering is optional
		ciphered := false
		switch msg.SecurityHeaderType {
		case nas.SecurityHeaderTypeIntegrityProtected:
			// ue.NASLog.Debugln("Security header type: Integrity Protected")
		case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
			// ue.NASLog.Debugln("Security header type: Integrity Protected And Ciphered")
			ciphered = true
		case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
			// ue.NASLog.Debugln("Security header type: Integrity Protected And Ciphered With New 5G Security Context")
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
				// ue.NASLog.Debugf("set ULCount overflow")
				ulCountNew.SetOverflow(ulCountNew.Overflow() + 1)
			}
			ulCountNew.SetSQN(sequenceNumber)

			// ue.NASLog.Debugf("Calculate NAS MAC (algorithm: %+v, ULCount: 0x%0x)", ue.IntegrityAlg, ulCountNew.Get())
			// ue.NASLog.Tracef("NAS integrity key0x: %0x", ue.KnasInt)
			var mac32 []byte
			mac32, err = security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ulCountNew.Get(),
				GetBearerType(accessType), security.DirectionUplink, payload)
			if err != nil {
				return nil, fmt.Errorf("MAC calcuate error: %+v", err)
			}

			if !reflect.DeepEqual(mac32, receivedMac32) {
				// ue.NASLog.Warnf("NAS MAC verification failed(received: 0x%08x, expected: 0x%08x)", receivedMac32, mac32)
			} else {
				// ue.NASLog.Tracef("cmac value: 0x%08x", mac32)
				integrityProtected = true
			}
		} else {
			ue.NASLog.Warn("UE Security Context is not Available, so skip MAC verify")
		}

		if ciphered {
			if !integrityProtected {
				return nil, fmt.Errorf("NAS message is ciphered, but MAC verification failed")
			}
			// ue.NASLog.Debugf("Decrypt NAS message (algorithm: %+v, ULCount: 0x%0x)", ue.CipheringAlg, ulCountNew.Get())
			// ue.NASLog.Tracef("NAS ciphering key: %0x", ue.KnasEnc)
			// decrypt payload without sequence number (payload[1])
			if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ulCountNew.Get(), GetBearerType(accessType),
				security.DirectionUplink, payload[1:]); err != nil {
				return nil, fmt.Errorf("decrypt error: %+v", err)
			}
		}

		// remove sequece Number
		payload = payload[1:]
	}

	err = msg.PlainNasDecode(&payload)
	if err != nil {
		return nil, err
	}

	msgTypeText := func() string {
		if msg.GmmMessage == nil {
			return "Non GMM message"
		} else {
			return fmt.Sprintf(" message type %d", msg.GmmHeader.GetMessageType())
		}
	}
	errNoSecurityContext := func() error {
		return fmt.Errorf("UE Security Context is not Available, %s", msgTypeText())
	}
	errWrongSecurityHeader := func() error {
		return fmt.Errorf("wrong security header type: 0x%0x, %s", msg.SecurityHeader.SecurityHeaderType, msgTypeText())
	}
	errMacVerificationFailed := func() error {
		return fmt.Errorf("MAC verification failed, %s", msgTypeText())
	}

	if msg.GmmMessage == nil {
		if !ue.SecurityContextAvailable {
			return nil, errNoSecurityContext()
		}
		if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
			return nil, errWrongSecurityHeader()
		}
		if !integrityProtected {
			return nil, errMacVerificationFailed()
		}
	} else {
		switch msg.GmmHeader.GetMessageType() {
		case nas.MsgTypeRegistrationRequest:
			if ue.SecurityContextAvailable {
				// Periodic/mobility updating and most re-registration cases
				if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered &&
					msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext {
					return nil, errWrongSecurityHeader()
				}
				if !integrityProtected {
					return nil, errMacVerificationFailed()
				}
			} else {
				// Fresh attach: plain is allowed; protected-with-new-context is also possible after SMC
				if msg.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
					// You may allow "WithNew5gNasSecurityContext" if you support it here
					return nil, errWrongSecurityHeader()
				}
			}
		case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
			if initialMessage {
				if msg.SecurityHeaderType == nas.SecurityHeaderTypeIntegrityProtectedAndCiphered ||
					msg.SecurityHeaderType == nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext {
					return nil, errWrongSecurityHeader()
				}
			} else {
				if ue.SecurityContextAvailable {
					if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
						return nil, errWrongSecurityHeader()
					}
					if !integrityProtected {
						return nil, errMacVerificationFailed()
					}
				}
			}
		case nas.MsgTypeServiceRequest:
			if initialMessage {
				if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtected {
					return nil, errWrongSecurityHeader()
				}
			} else {
				if !ue.SecurityContextAvailable {
					return nil, errNoSecurityContext()
				}
				if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
					return nil, errWrongSecurityHeader()
				}
				if !integrityProtected {
					return nil, errMacVerificationFailed()
				}
			}
		case nas.MsgTypeIdentityResponse:
			mobileIdentityContents := msg.IdentityResponse.MobileIdentity.GetMobileIdentityContents()
			if len(mobileIdentityContents) >= 1 &&
				nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) == nasMessage.MobileIdentity5GSTypeSuci {
				// Identity is SUCI
				if ue.SecurityContextAvailable {
					if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
						return nil, errWrongSecurityHeader()
					}
					if !integrityProtected {
						return nil, errMacVerificationFailed()
					}
				}
			} else {
				// Identity is not SUCI
				if !ue.SecurityContextAvailable {
					return nil, errNoSecurityContext()
				}
				if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
					return nil, errWrongSecurityHeader()
				}
				if !integrityProtected {
					return nil, errMacVerificationFailed()
				}
			}
		case nas.MsgTypeAuthenticationResponse,
			nas.MsgTypeAuthenticationFailure,
			nas.MsgTypeSecurityModeReject,
			nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
			if ue.SecurityContextAvailable {
				if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
					return nil, errWrongSecurityHeader()
				}
				if !integrityProtected {
					return nil, errMacVerificationFailed()
				}
			}
		case nas.MsgTypeSecurityModeComplete:
			if !ue.SecurityContextAvailable {
				return nil, errNoSecurityContext()
			}
			if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext {
				return nil, errWrongSecurityHeader()
			}
			if !integrityProtected {
				return nil, errMacVerificationFailed()
			}
		default:
			if !ue.SecurityContextAvailable {
				return nil, errNoSecurityContext()
			}
			if msg.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
				return nil, errWrongSecurityHeader()
			}
			if !integrityProtected {
				return nil, errMacVerificationFailed()
			}
		}
	}

	if integrityProtected {
		ue.ULCount = ulCountNew
	}

	logger.AmfLog.Warn("TO DELETE: NAS message decoded successfully", zap.Int("ULCount", int(ue.ULCount.Get())), zap.Bool("integrityProtected", integrityProtected))

	return msg, nil
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
