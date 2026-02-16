package validate

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/core-tester/internal/gnb"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests/utils"
	"github.com/ellanetworks/core/core-tester/internal/ue"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceSetupRequestOpts struct {
	Frame                gnb.SCTPFrame
	ExpectedPDUSessionID uint8
	ExpectedSST          int32
	ExpectedSD           string
	UEIns                *ue.UE
}

func PDUSessionResourceSetupRequest(opts *PDUSessionResourceSetupRequestOpts) error {
	err := utils.ValidateSCTP(opts.Frame.Info, 60, 1)
	if err != nil {
		return fmt.Errorf("SCTP validation failed: %v", err)
	}

	pdu, err := ngap.Decoder(opts.Frame.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	if pdu.InitiatingMessage == nil {
		return fmt.Errorf("NGAP PDU is not a InitiatingMessage")
	}

	if pdu.InitiatingMessage.ProcedureCode.Value != ngapType.ProcedureCodePDUSessionResourceSetup {
		return fmt.Errorf("NGAP PDU is not a PDUSessionResourceSetupRequest")
	}

	pDUSessionResourceSetupRequest := pdu.InitiatingMessage.Value.PDUSessionResourceSetupRequest
	if pDUSessionResourceSetupRequest == nil {
		return fmt.Errorf("PDUSessionResourceSetupRequest is nil")
	}

	var (
		amfueNGAPID                                  *ngapType.AMFUENGAPID
		ranueNGAPID                                  *ngapType.RANUENGAPID
		protocolIEIDPDUSessionResourceSetupListSUReq *ngapType.PDUSessionResourceSetupListSUReq
		protocolIEIDUEAggregateMaximumBitRate        *ngapType.UEAggregateMaximumBitRate
	)

	for _, ie := range pDUSessionResourceSetupRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			amfueNGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID:
			ranueNGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
			protocolIEIDPDUSessionResourceSetupListSUReq = ie.Value.PDUSessionResourceSetupListSUReq
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			protocolIEIDUEAggregateMaximumBitRate = ie.Value.UEAggregateMaximumBitRate
		default:
			return fmt.Errorf("PDUSessionResourceSetupRequest IE ID (%d) not supported", ie.Id.Value)
		}
	}

	if amfueNGAPID == nil {
		return fmt.Errorf("AMFUENGAPID is missing in PDUSessionResourceSetupRequest")
	}

	if ranueNGAPID == nil {
		return fmt.Errorf("RANUENGAPID is missing in PDUSessionResourceSetupRequest")
	}

	if protocolIEIDPDUSessionResourceSetupListSUReq == nil {
		return fmt.Errorf("PDUSessionResourceSetupListSUReq is missing in PDUSessionResourceSetupRequest")
	}

	if protocolIEIDUEAggregateMaximumBitRate == nil {
		return fmt.Errorf("UEAggregateMaximumBitRate is missing in PDUSessionResourceSetupRequest")
	}

	err = pduSessionResourceSetupListSUReq(protocolIEIDPDUSessionResourceSetupListSUReq, opts.ExpectedPDUSessionID, opts.ExpectedSST, opts.ExpectedSD, opts.UEIns)
	if err != nil {
		return fmt.Errorf("PDUSessionResourceSetupListSUReq validation failed: %v", err)
	}

	return nil
}

func pduSessionResourceSetupListSUReq(
	pDUSessionResourceSetupListSUReq *ngapType.PDUSessionResourceSetupListSUReq,
	expectedPDUSessionID uint8,
	expectedSST int32,
	expectedSD string,
	ueIns *ue.UE,
) error {
	if len(pDUSessionResourceSetupListSUReq.List) != 1 {
		return fmt.Errorf("PDUSessionResourceSetupListSUReq should have exactly one item, got: %d", len(pDUSessionResourceSetupListSUReq.List))
	}

	item := pDUSessionResourceSetupListSUReq.List[0]
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

	if item.PDUSessionNASPDU == nil {
		return fmt.Errorf("PDUSessionNASPDU is nil")
	}

	msg, err := ueIns.DecodeNAS(item.PDUSessionNASPDU.Value)
	if err != nil {
		return fmt.Errorf("could not decode PDU Session NAS PDU: %v", err)
	}

	if msg.GmmMessage == nil {
		return fmt.Errorf("NAS message is not a GMM message")
	}

	if msg.GmmMessage.GetMessageType() != nas.MsgTypeDLNASTransport {
		return fmt.Errorf("NAS message type is not DLNASTransport (%d), got (%d)", nas.MsgTypeDLNASTransport, msg.GmmMessage.GetMessageType())
	}

	if msg.DLNASTransport == nil {
		return fmt.Errorf("NAS DLNASTransport message is nil")
	}

	if reflect.ValueOf(msg.DLNASTransport.ExtendedProtocolDiscriminator).IsZero() {
		return fmt.Errorf("extended protocol is missing")
	}

	if msg.DLNASTransport.GetExtendedProtocolDiscriminator() != 126 {
		return fmt.Errorf("extended protocol not the expected value")
	}

	if msg.DLNASTransport.GetSpareHalfOctet() != 0 {
		return fmt.Errorf("spare half not expected value")
	}

	if msg.DLNASTransport.GetSecurityHeaderType() != 0 {
		return fmt.Errorf("security header not expected value")
	}

	if reflect.ValueOf(msg.DLNASTransport.SpareHalfOctetAndPayloadContainerType).IsZero() {
		return fmt.Errorf("payload container type is missing")
	}

	if msg.DLNASTransport.GetPayloadContainerType() != 1 {
		return fmt.Errorf("payload container type not expected value")
	}

	if reflect.ValueOf(msg.DLNASTransport.PayloadContainer).IsZero() || msg.DLNASTransport.GetPayloadContainerContents() == nil {
		return fmt.Errorf("payload container is missing")
	}

	if reflect.ValueOf(msg.DLNASTransport.PduSessionID2Value).IsZero() {
		return fmt.Errorf("pdu session id is missing")
	}

	if msg.DLNASTransport.PduSessionID2Value.GetIei() != 18 {
		return fmt.Errorf("pdu session id not expected value")
	}

	err = pduSessionResourceSetupTransfer(item.PDUSessionResourceSetupRequestTransfer)
	if err != nil {
		return fmt.Errorf("could not validate PDU Session Resource Setup Transfer: %v", err)
	}

	return nil
}

type PDUSessionResourceSetupRequestTransfer struct {
	ULTeid     uint32
	UpfAddress string
	QosId      int64
	FiveQi     int64
	PriArp     int64
	PduSType   uint64
}

func pduSessionResourceSetupTransfer(transfer aper.OctetString) error {
	if transfer == nil {
		return fmt.Errorf("PDU Session Resource Setup Request Transfer is missing")
	}

	pdu := &ngapType.PDUSessionResourceSetupRequestTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return fmt.Errorf("could not unmarshal Pdu Session Resource Setup Request Transfer: %v", err)
	}

	return nil
}
