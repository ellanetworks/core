// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/free5gc/nas/nasMessage"
)

func handleAuthenticationFailure(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.AuthenticationFailure) error {
	if state := ue.GetState(); state != amf.Authentication {
		return fmt.Errorf("state mismatch: receive amf.Authentication Failure message in state %s", state)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	conn.T3560.Stop()

	switch msg.GetCauseValue() {
	case nasMessage.Cause5GMMMACFailure:
		ue.Log.Warn("amf.Authentication Failure Cause: Mac Failure")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ranUe)

		return nil
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		ue.Log.Warn("amf.Authentication Failure Cause: Non-5G amf.Authentication Unacceptable")
		ue.Deregister(ctx)

		amf.SendAuthenticationReject(ctx, ranUe)

		return nil
	case nasMessage.Cause5GMMngKSIAlreadyInUse:
		ue.Log.Warn("amf.Authentication Failure Cause: NgKSI Already In Use")

		conn.AuthFailureCauseSynchFailureTimes = 0

		ue.Log.Warn("Select new NgKsi")

		ngKsi := ue.NgKsi()
		ngKsi.Ksi = amf.NextNgKsi(ngKsi.Ksi)
		ue.SetNgKsi(ngKsi)

		amf.SendAuthenticationRequest(ctx, amfInstance, ranUe)

		ue.Log.Info("Sent authentication request")
	case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
		ue.Log.Warn("amf.Authentication Failure 5GMM Cause: Synch Failure")

		conn.AuthFailureCauseSynchFailureTimes++
		if conn.AuthFailureCauseSynchFailureTimes >= 2 {
			ue.Log.Warn("2 consecutive Synch Failure, terminate authentication procedure")
			ue.Deregister(ctx)

			amf.SendAuthenticationReject(ctx, ranUe)

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
			return fmt.Errorf("send UE amf.Authentication Authenticate Request Error: %s", err.Error())
		}

		conn.AuthenticationCtx = response

		ue.SetAbba([]uint8{0x00, 0x00})

		amf.SendAuthenticationRequest(ctx, amfInstance, ranUe)

		ue.Log.Info("Sent authentication request")
	}

	return nil
}
