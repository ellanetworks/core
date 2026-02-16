package validate

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type InitialContextSetupRequestOpts struct {
	Frame gnb.SCTPFrame
}

type InitialContextSetupRequestResp struct {
	AMFUENGAPID                       *ngapType.AMFUENGAPID
	RANUENGAPID                       *ngapType.RANUENGAPID
	PDUSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq
	NASPDU                            *ngapType.NASPDU
}

func InitialContextSetupRequest(opts *InitialContextSetupRequestOpts) (*InitialContextSetupRequestResp, error) {
	err := utils.ValidateSCTP(opts.Frame.Info, 60, 1)
	if err != nil {
		return nil, fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngap.Decoder(opts.Frame.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.InitiatingMessage == nil {
		return nil, fmt.Errorf("NGAP PDU is not a InitiatingMessage")
	}

	if pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodeInitialContextSetup {
		return nil, fmt.Errorf("NGAP ProcedureCode is not InitialContextSetup (%d), received %d", ngapType.ProcedureCodeInitialContextSetup, pdu.InitiatingMessage.ProcedureCode.Value)
	}

	initialContextSetupRequest := pdu.InitiatingMessage.Value.InitialContextSetupRequest
	if initialContextSetupRequest == nil {
		return nil, fmt.Errorf("InitialContextSetupRequest is nil")
	}

	var (
		amfueNGAPID                                   *ngapType.AMFUENGAPID
		ranueNGAPID                                   *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq
		nasPDU                                        *ngapType.NASPDU
	)

	for _, ie := range initialContextSetupRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
			protocolIEIDPDUSessionResourceSetupListCxtReq = ie.Value.PDUSessionResourceSetupListCxtReq
		case ngapType.ProtocolIEIDGUAMI:
		case ngapType.ProtocolIEIDAllowedNSSAI:
		case ngapType.ProtocolIEIDUESecurityCapabilities:
		case ngapType.ProtocolIEIDSecurityKey:
		case ngapType.ProtocolIEIDNASPDU:
			nasPDU = ie.Value.NASPDU
		case ngapType.ProtocolIEIDMobilityRestrictionList:
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		default:
			return nil, fmt.Errorf("PDUSessionResourceSetupRequest IE ID (%d) not supported", ie.Id.Value)
		}
	}

	msgResp := &InitialContextSetupRequestResp{
		AMFUENGAPID:                       amfueNGAPID,
		RANUENGAPID:                       ranueNGAPID,
		NASPDU:                            nasPDU,
		PDUSessionResourceSetupListCxtReq: protocolIEIDPDUSessionResourceSetupListCxtReq,
	}

	return msgResp, nil
}

func PDUSessionResourceSetupListCxtReq(
	pDUSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq,
	expectedPDUSessionID uint8,
	expectedSST int32,
	expectedSD string,
) error {
	if len(pDUSessionResourceSetupListCxtReq.List) != 1 {
		logger.GnbLogger.Error("got", zap.Any("pducxtlist", pDUSessionResourceSetupListCxtReq.List))
		return fmt.Errorf("PDUSessionResourceSetupListCxtReq should have exactly one item, got: %d", len(pDUSessionResourceSetupListCxtReq.List))
	}

	item := pDUSessionResourceSetupListCxtReq.List[0]
	if item.PDUSessionID.Value != int64(expectedPDUSessionID) {
		return fmt.Errorf("unexpected PDUSessionID: %d", item.PDUSessionID.Value)
	}

	expectedSSTBytes, expectedSDBytes, err := gnb.GetSliceInBytes(expectedSST, expectedSD)
	if err != nil {
		return fmt.Errorf("could not convert expected SST and SD to byte slices: %v", err)
	}

	if !bytes.Equal(item.SNSSAI.SST.Value, expectedSSTBytes) {
		return fmt.Errorf("unexpected SNSSAI SST: %x, expected: %x", item.SNSSAI.SST.Value, expectedSSTBytes)
	}

	if !bytes.Equal(item.SNSSAI.SD.Value, expectedSDBytes) {
		return fmt.Errorf("unexpected SNSSAI SD: %x, expected: %x", item.SNSSAI.SD.Value, expectedSDBytes)
	}

	err = pduSessionResourceSetupTransfer(item.PDUSessionResourceSetupRequestTransfer)
	if err != nil {
		return fmt.Errorf("could not validate PDU Session Resource Setup Transfer: %v", err)
	}

	return nil
}
