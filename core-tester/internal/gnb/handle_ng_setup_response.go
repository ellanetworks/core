package gnb

import (
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleNGSetupResponse(nGSetupResponse *ngapType.NGSetupResponse) error {
	var (
		amfName             *ngapType.AMFName
		guamiList           *ngapType.ServedGUAMIList
		relativeAMFCapacity *ngapType.RelativeAMFCapacity
		plmnSupportList     *ngapType.PLMNSupportList
	)

	for _, ie := range nGSetupResponse.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			amfName = ie.Value.AMFName
		case ngapType.ProtocolIEIDServedGUAMIList:
			guamiList = ie.Value.ServedGUAMIList
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			relativeAMFCapacity = ie.Value.RelativeAMFCapacity
		case ngapType.ProtocolIEIDPLMNSupportList:
			plmnSupportList = ie.Value.PLMNSupportList
		}
	}

	logger.GnbLogger.Debug(
		"Received NGSetupResponse",
		zap.String("AMFName", amfName.Value),
		zap.Int("GUAMIListCount", len(guamiList.List)),
		zap.Int("RelativeAMFCapacity", int(relativeAMFCapacity.Value)),
		zap.Int("PLMNSupportListCount", len(plmnSupportList.List)),
	)

	return nil
}
