// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
)

// recordLPPCapability latches the UE's LPP-in-N1-mode bit from the 5GMM
// capability IE (TS 24.501 §9.11.3.1).
//
// The IE is not a cleartext IE, so a UE without a security context omits it from
// its first REGISTRATION REQUEST and re-sends the whole message inside the NAS
// message container of SECURITY MODE COMPLETE (§4.4.6). Both arrivals land here,
// and an absent IE never overwrites a bit already learned.
func recordLPPCapability(ue *amf.UeContext, msg *nasMessage.RegistrationRequest) {
	if msg == nil || msg.Capability5GMM == nil {
		return
	}

	lpp := msg.Capability5GMM.GetLPP() == 1
	ue.LPPN1Supported = &lpp

	// 5G-LCS is octet 4 (Octet[1]) bit 6 of the 5GMM capability IE
	// (TS 24.501 §9.11.3.1): "LCS notification mechanisms supported".
	lcs := msg.Capability5GMM.Octet[1]&0x20 != 0
	ue.LCSNotificationSupported = &lcs
}

func contextSetup(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.RegistrationRequest) {
	ctx, span := gmmTracer.Start(ctx, "nas/context_setup")
	defer span.End()

	ue.AdvanceRegStep(amf.RegStepContextSetup)

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	conn.RegistrationRequest = msg

	recordLPPCapability(ue, msg)

	switch conn.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		HandleInitialRegistration(ctx, amfInstance, ue)
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfInstance, ue)
	}
}
