package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func handleDownlinkNASTransport(gnb *GnodeB, downlinkNASTransport *ngapType.DownlinkNASTransport) error {
	var (
		amfUENGAPID *ngapType.AMFUENGAPID
		ranUENGAPID *ngapType.RANUENGAPID
		nasPDU      *ngapType.NASPDU
	)

	for _, ie := range downlinkNASTransport.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranUENGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDNASPDU:
			nasPDU = ie.Value.NASPDU
		}
	}

	if amfUENGAPID == nil {
		return fmt.Errorf("missing AMF UE NGAP ID in DownlinkNASTransport")
	}

	if ranUENGAPID == nil {
		return fmt.Errorf("missing RAN UE NGAP ID in DownlinkNASTransport")
	}

	if nasPDU == nil {
		return fmt.Errorf("missing NAS PDU in DownlinkNASTransport")
	}

	logger.GnbLogger.Debug("Received DownlinkNASTransport",
		zap.Int64("AMFUENGAPID", amfUENGAPID.Value),
		zap.Int64("RANUENGAPID", ranUENGAPID.Value),
	)

	gnb.UpdateNGAPIDs(ranUENGAPID.Value, amfUENGAPID.Value)

	ue, err := gnb.LoadUE(ranUENGAPID.Value)
	if err != nil {
		return fmt.Errorf("cannot find UE for DownlinkNASTransport message: %v", err)
	}

	err = ue.SendDownlinkNAS(nasPDU.Value, amfUENGAPID.Value, ranUENGAPID.Value)
	if err != nil {
		return fmt.Errorf("HandleDownlinkNASTransport failed: %v", err)
	}

	return nil
}
