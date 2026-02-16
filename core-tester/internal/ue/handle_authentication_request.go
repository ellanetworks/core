package ue

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func handleAuthenticationRequest(ue *UE, msg *nas.Message, amfUENGAPID int64, ranUENGAPID int64) error {
	logger.UeLogger.Debug("Received Authentication Request NAS message")

	rand := msg.GetRANDValue()
	autn := msg.GetAUTN()

	paramAutn, err := ue.DeriveRESstarAndSetKey(ue.UeSecurity.AuthenticationSubs, rand[:], ue.UeSecurity.Snn, autn[:])
	if err != nil {
		return fmt.Errorf("could not derive RES* and set key: %v", err)
	}

	authResp, err := BuildAuthenticationResponse(&AuthenticationResponseOpts{
		AuthenticationResponseParam: paramAutn,
		EapMsg:                      "",
	})
	if err != nil {
		return fmt.Errorf("could not build authentication response: %v", err)
	}

	err = ue.Gnb.SendUplinkNAS(authResp, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send Authentication Response: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Authentication Response NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
