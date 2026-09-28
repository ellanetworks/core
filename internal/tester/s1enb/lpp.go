// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

type LPPFix struct {
	Latitude           int32
	Longitude          int32
	Altitude           int32
	HorizontalAccuracy uint32
	VerticalAccuracy   uint32
}

func (e *ENB) AnswerLPP(ue *UE, enbUEID int64, fix LPPFix, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out awaiting LPP RequestLocationInformation")
		}

		wire, mmeUEID, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return err
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return fmt.Errorf("unprotect downlink NAS: %w", err)
		}

		msg, err := parseDownlink(plain)
		if err != nil {
			return err
		}

		dl, ok := msg.(*eps.DownlinkGenericNASTransport)
		if !ok || dl.ContainerType != eps.GenericMessageContainerTypeLPP {
			continue
		}

		decoded, err := lpp.DecodeLPPMessage(dl.Container)
		if err != nil {
			return fmt.Errorf("decode LPP message: %w", err)
		}

		var reply []byte

		switch decoded.BodyKind {
		case lpptype.LPPMessageBodyC1PresentRequestCapabilities:
			reply, err = lpp.EncodeProvideCapabilities(decoded.TransactionID, []int64{lpptype.GNSSIDGPS, lpptype.GNSSIDGLONASS})
		case lpptype.LPPMessageBodyC1PresentRequestLocationInformation:
			reply, err = lpp.EncodeProvideLocationInformation(decoded.TransactionID, fix.Latitude, fix.Longitude, fix.Altitude, fix.HorizontalAccuracy, fix.VerticalAccuracy)
		case lpptype.LPPMessageBodyC1PresentAbort, lpptype.LPPMessageBodyC1PresentError:
			return fmt.Errorf("received LPP abort or error from the LMF")
		default:
			continue
		}

		if err != nil {
			return fmt.Errorf("build LPP reply: %w", err)
		}

		if err := e.sendUplinkLPP(ue, mmeUEID, enbUEID, dl.AdditionalInformation, reply); err != nil {
			return err
		}

		logger.GnbLogger.Debug("s1enb: answered LPP request", zap.Int("body_kind", decoded.BodyKind))

		if decoded.BodyKind == lpptype.LPPMessageBodyC1PresentRequestLocationInformation {
			return nil
		}
	}
}

func (e *ENB) sendUplinkLPP(ue *UE, mmeUEID, enbUEID int64, correlationID, lppMsg []byte) error {
	plain, err := (&eps.UplinkGenericNASTransport{
		ContainerType:         eps.GenericMessageContainerTypeLPP,
		Container:             lppMsg,
		AdditionalInformation: correlationID,
	}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Uplink Generic NAS Transport: %w", err)
	}

	wire, err := ue.protectUplink(plain)
	if err != nil {
		return fmt.Errorf("protect Uplink Generic NAS Transport: %w", err)
	}

	if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, wire); err != nil {
		return fmt.Errorf("send Uplink Generic NAS Transport: %w", err)
	}

	return nil
}
