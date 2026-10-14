// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const registrationCompleteTimeout = 5 * time.Second

func handleRegistrationAccept(ue *UE, plain []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Registration Accept NAS message", zap.String("IMSI", ue.UeSecurity.Supi))

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fmt.Errorf("could not parse Registration Accept: %v", err)
	}

	if regAccept.GUTI != nil {
		ue.Set5gGuti(regAccept.GUTI)
	}

	ue.smsAllowed.Store(regAccept.SMSAllowed)
	ue.smsAvailable.Store(false)

	regComplete, err := BuildRegistrationComplete(&RegistrationCompleteOpts{
		SORTransparentContainer: nil,
	})
	if err != nil {
		return fmt.Errorf("could not build Registration Complete NAS PDU: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(regComplete, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Registration Complete Msg", ue.UeSecurity.Supi)
	}

	cucSeen := ue.seenGMM(uint8(fgs.MsgConfigurationUpdateCommand))

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Registration Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	if ue.skipAutoPDUSession() {
		return nil
	}

	go func() {
		if !ue.waitSeenGMMAbove(uint8(fgs.MsgConfigurationUpdateCommand), cucSeen, registrationCompleteTimeout) {
			logger.UeLogger.Warn("no Configuration Update Command after Registration Complete; sending the PDU Session Establishment Request anyway",
				zap.String("IMSI", ue.UeSecurity.Supi))
		}

		if err := sendPDUSessionEstablishmentRequest(ue, amfUENGAPID, ranUENGAPID); err != nil {
			logger.UeLogger.Error("could not send PDU Session Establishment Request", zap.String("IMSI", ue.UeSecurity.Supi), zap.Error(err))
		}
	}()

	return nil
}

func sendPDUSessionEstablishmentRequest(ue *UE, amfUENGAPID int64, ranUENGAPID int64) error {
	pduReq, err := BuildPduSessionEstablishmentRequest(&PduSessionEstablishmentRequestOpts{
		PDUSessionID:   ue.PDUSessionID,
		PDUSessionType: ue.PDUSessionType,
	})
	if err != nil {
		return fmt.Errorf("could not build PDU Session Establishment Request: %v", err)
	}

	pduUplink, err := BuildUplinkNasTransport(&UplinkNasTransportOpts{
		PDUSessionID:     ue.PDUSessionID,
		PayloadContainer: pduReq,
		DNN:              ue.DNN,
		SNSSAI:           ue.Snssai,
	})
	if err != nil {
		return fmt.Errorf("could not build Uplink NAS Transport for PDU Session: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(pduUplink, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Uplink NAS Transport for PDU Session Msg", ue.UeSecurity.Supi)
	}

	if err := ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("could not send UplinkNASTransport for PDU Session Establishment: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Establishment Request",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
