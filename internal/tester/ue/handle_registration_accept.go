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

func handleRegistrationAccept(ue *UE, plain []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Registration Accept NAS message", zap.String("IMSI", ue.UeSecurity.Supi))

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		return fmt.Errorf("could not parse Registration Accept: %v", err)
	}

	if regAccept.GUTI != nil {
		ue.Set5gGuti(regAccept.GUTI)
	}

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

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	// Registration Complete has no response; pause so the Core finishes processing
	// it before the PDU Session Establishment Request arrives.
	time.Sleep(500 * time.Millisecond)

	logger.UeLogger.Debug(
		"Sent Registration Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

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

	encodedPdu, err = ue.EncodeNasPduWithSecurity(pduUplink, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Uplink NAS Transport for PDU Session Msg", ue.UeSecurity.Supi)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport for PDU Session Establishment: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Establishment Request",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
