// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleUEContextReleaseCommand(gnb *GnodeB, value []byte) error {
	cmd, err := ngap.ParseUEContextReleaseCommand(value)
	if err != nil {
		return fmt.Errorf("could not parse UEContextReleaseCommand: %w", err)
	}

	// The AMF may address the UE by the pair or by the AMF UE NGAP ID alone
	// (TS 38.413 §9.2.2.5); only the pair names a RAN UE this gNB can resolve.
	if !cmd.UENGAPIDs.Pair {
		return fmt.Errorf("UEContextReleaseCommand carries no RAN UE NGAP ID")
	}

	amfUEID, ranUEID := int64(cmd.UENGAPIDs.AMFUENGAPID), int64(cmd.UENGAPIDs.RANUENGAPID)

	logger.GnbLogger.Debug("Received UE Context Release Command",
		zap.String("Cause", causeName(cmd.Cause)),
		zap.Int64("RAN UE NGAP ID", ranUEID),
		zap.Int64("AMF UE NGAP ID", amfUEID),
	)

	ue, err := gnb.LoadUE(ranUEID)
	if err != nil {
		return fmt.Errorf("cannot find UE for UEContextReleaseCommand message: %v", err)
	}

	var released [16]bool

	for _, session := range gnb.pduSessionsFor(ranUEID) {
		if session.PDUSessionID >= 0 && int(session.PDUSessionID) < len(released) {
			released[session.PDUSessionID] = true
		}
	}

	ue.RRCRelease()

	err = gnb.SendUEContextReleaseComplete(&UEContextReleaseCompleteOpts{
		AMFUENGAPID:   amfUEID,
		RANUENGAPID:   ranUEID,
		PDUSessionIDs: released,
		Mcc:           gnb.MCC,
		Mnc:           gnb.MNC,
		GnbID:         gnb.GnbID,
		Tac:           gnb.TAC,
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	gnb.dropPDUSessions(ranUEID)
	gnb.dropRadioCapabilityReport(ranUEID)

	logger.GnbLogger.Debug(
		"Sent UE Context Release Complete",
		zap.Int64("RAN UE NGAP ID", ranUEID),
		zap.Int64("AMF UE NGAP ID", amfUEID),
	)

	return nil
}

// causeName renders a Cause the AMF may legitimately omit (ignore criticality).
func causeName(cause *ngap.Cause) string {
	if cause == nil {
		return "absent"
	}

	return cause.String()
}
