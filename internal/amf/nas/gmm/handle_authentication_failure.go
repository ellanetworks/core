package gmm

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/free5gc/nas/nasMessage"
)

func handleAuthenticationFailure(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nasMessage.AuthenticationFailure) error {
	if state := ue.GetState(); state != amf.Authentication {
		return fmt.Errorf("state mismatch: receive Authentication Failure message in state %s", state)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	if ue.NasConn().T3560 != nil {
		ue.NasConn().T3560.Stop()
		ue.NasConn().T3560 = nil // clear the timer
	}

	switch msg.GetCauseValue() {
	case nasMessage.Cause5GMMMACFailure:
		ue.Log.Warn("Authentication Failure Cause: Mac Failure")
		ue.Deregister(ctx)

		err := message.SendAuthenticationReject(ctx, ranUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		ue.Log.Warn("Authentication Failure Cause: Non-5G Authentication Unacceptable")
		ue.Deregister(ctx)

		err := message.SendAuthenticationReject(ctx, ranUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return nil
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		ue.Log.Warn("Authentication Failure Cause: NgKSI Already In Use")
		ue.NasConn().AuthFailureCauseSynchFailureTimes = 0
		ue.Log.Warn("Select new NgKsi")
		ue.Current().NgKsi.Ksi = nextNgKsi(ue.Current().NgKsi.Ksi)

		err := message.SendAuthenticationRequest(ctx, amfInstance, ranUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.Log.Info("Sent authentication request")
	case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
		ue.Log.Warn("Authentication Failure 5GMM Cause: Synch Failure")

		ue.NasConn().AuthFailureCauseSynchFailureTimes++
		if ue.NasConn().AuthFailureCauseSynchFailureTimes >= 2 {
			ue.Log.Warn("2 consecutive Synch Failure, terminate authentication procedure")
			ue.Deregister(ctx)

			err := message.SendAuthenticationReject(ctx, ranUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return nil
		}

		if msg.AuthenticationFailureParameter == nil {
			return fmt.Errorf("missing AuthenticationFailureParameter IE for SynchFailure")
		}

		auts := msg.GetAuthenticationFailureParameter()
		resynchronizationInfo := &ausf.ResyncInfo{
			Auts: hex.EncodeToString(auts[:]),
		}

		response, err := sendUEAuthenticationAuthenticateRequest(ctx, amfInstance, ue, resynchronizationInfo)
		if err != nil {
			return fmt.Errorf("send UE Authentication Authenticate Request Error: %s", err.Error())
		}

		ue.NasConn().AuthenticationCtx = response
		ue.Current().ABBA = []uint8{0x00, 0x00}

		err = message.SendAuthenticationRequest(ctx, amfInstance, ranUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.Log.Info("Sent authentication request")
	}

	return nil
}
