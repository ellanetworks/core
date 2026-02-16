package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleUEContextReleaseCommand(gnb *GnodeB, uEContextReleaseCommand *ngapType.UEContextReleaseCommand) error {
	var (
		cause     *ngapType.Cause
		ueNgapIDs *ngapType.UENGAPIDs
	)

	for _, ie := range uEContextReleaseCommand.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDUENGAPIDs:
			ueNgapIDs = ie.Value.UENGAPIDs
		}
	}

	logger.GnbLogger.Debug("Received UE Context Release Command",
		zap.String("Cause", causeToString(*cause)),
		zap.Any("UE NGAP IDs", ueNgapIDs),
	)

	ue, err := gnb.LoadUE(ueNgapIDs.UENGAPIDPair.RANUENGAPID.Value)
	if err != nil {
		return fmt.Errorf("cannot find UE for UEContextReleaseCommand message: %v", err)
	}

	ue.RRCRelease()

	err = gnb.SendUEContextReleaseComplete(&UEContextReleaseCompleteOpts{
		AMFUENGAPID: ueNgapIDs.UENGAPIDPair.AMFUENGAPID.Value,
		RANUENGAPID: ueNgapIDs.UENGAPIDPair.RANUENGAPID.Value,
	})
	if err != nil {
		return fmt.Errorf("could not send UEContextReleaseComplete: %v", err)
	}

	logger.GnbLogger.Debug(
		"Sent UE Context Release Complete",
		zap.Int64("RAN UE NGAP ID", ueNgapIDs.UENGAPIDPair.RANUENGAPID.Value),
		zap.Int64("AMF UE NGAP ID", ueNgapIDs.UENGAPIDPair.AMFUENGAPID.Value),
	)

	return nil
}
