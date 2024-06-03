package util

import (
	"net"
	"time"

	"github.com/omec-project/aper"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/ngapType"
)

// ASN.1 Basic-PER encoded values
const (
	NgapPDUIncomingMessage     byte = 0x00
	NgapPDUSuccessfulOutcome   byte = 0x20
	NgapPDUUnSuccessfulOutcome byte = 0x40
)

var MessageTypeMap = map[byte]string{
	NgapPDUIncomingMessage:     "IncomingMessage",
	NgapPDUSuccessfulOutcome:   "SuccessfulOutcome",
	NgapPDUUnSuccessfulOutcome: "UnsuccessfulOutcome",
}

// Mock Connection struct. Implements the net.Conn interface
type TestConn struct {
	Data []byte
}

type TestConnAddr struct{}

func (tca TestConnAddr) Network() (a string) { return }
func (tca TestConnAddr) String() (a string)  { return }

// Write method of the mocked testConn struct will be invoked as a part of the
// unit test framework
func (tc *TestConn) Write(b []byte) (n int, err error) {
	tc.Data = b
	return
}

func (tc *TestConn) Close() (e error) { return }

func (tc *TestConn) Read(b []byte) (n int, err error) { return }

func (tc *TestConn) LocalAddr() net.Addr                    { return TestConnAddr{} }
func (tc *TestConn) RemoteAddr() net.Addr                   { return TestConnAddr{} }
func (tc *TestConn) SetDeadline(t time.Time) (e error)      { return }
func (tc *TestConn) SetReadDeadline(t time.Time) (e error)  { return }
func (tc *TestConn) SetWriteDeadline(t time.Time) (e error) { return }

// GetNGSetupRequest returns an encoded NGSetupRequest based on the input parameters
func GetNGSetupRequest(gnbId []byte, bitlength uint64, name, tac string) ([]byte, error) {
	message := BuildNGSetupRequest()
	// GlobalRANNodeID
	ie := message.InitiatingMessage.Value.NGSetupRequest.ProtocolIEs.List[0]
	gnbID := ie.Value.GlobalRANNodeID.GlobalGNBID.GNBID.GNBID
	gnbID.Bytes = gnbId
	gnbID.BitLength = bitlength
	// RANNodeName
	ie = message.InitiatingMessage.Value.NGSetupRequest.ProtocolIEs.List[1]
	ie.Value.RANNodeName.Value = name

	ie = message.InitiatingMessage.Value.NGSetupRequest.ProtocolIEs.List[2]
	ie.Value.SupportedTAList.List[0].TAC.Value = aper.OctetString(tac)

	return ngap.Encoder(message)
}

// BuildNGSetupRequest forms and returns a new NGAPPDU struct value for
// NGSetupRequest populated with default values.
func BuildNGSetupRequest() (pdu ngapType.NGAPPDU) {
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
	initiatingMessage.Value.NGSetupRequest = new(ngapType.NGSetupRequest)

	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	nGSetupRequestIEs := &nGSetupRequest.ProtocolIEs

	// GlobalRANNodeID
	ie := ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGlobalRANNodeID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentGlobalRANNodeID
	ie.Value.GlobalRANNodeID = new(ngapType.GlobalRANNodeID)

	globalRANNodeID := ie.Value.GlobalRANNodeID
	globalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	globalRANNodeID.GlobalGNBID = new(ngapType.GlobalGNBID)

	globalGNBID := globalRANNodeID.GlobalGNBID
	globalGNBID.PLMNIdentity.Value = aper.OctetString("\x02\xf8\x39")
	globalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	globalGNBID.GNBID.GNBID = new(aper.BitString)

	gNBID := globalGNBID.GNBID.GNBID

	*gNBID = aper.BitString{
		Bytes:     []byte{0x45, 0x46, 0x47},
		BitLength: 24,
	}
	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// RANNodeName
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANNodeName
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentRANNodeName
	ie.Value.RANNodeName = new(ngapType.RANNodeName)

	rANNodeName := ie.Value.RANNodeName
	rANNodeName.Value = "free5GC"
	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
	// SupportedTAList
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDSupportedTAList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentSupportedTAList
	ie.Value.SupportedTAList = new(ngapType.SupportedTAList)

	supportedTAList := ie.Value.SupportedTAList

	// SupportedTAItem in SupportedTAList
	supportedTAItem := ngapType.SupportedTAItem{}
	supportedTAItem.TAC.Value = aper.OctetString("\x00\x00\x01")

	broadcastPLMNList := &supportedTAItem.BroadcastPLMNList
	// BroadcastPLMNItem in BroadcastPLMNList
	broadcastPLMNItem := ngapType.BroadcastPLMNItem{}
	broadcastPLMNItem.PLMNIdentity.Value = aper.OctetString("\x02\xf8\x39")

	sliceSupportList := &broadcastPLMNItem.TAISliceSupportList
	// SliceSupportItem in SliceSupportList
	sliceSupportItem := ngapType.SliceSupportItem{}
	sliceSupportItem.SNSSAI.SST.Value = aper.OctetString("\x01")
	// optional
	sliceSupportItem.SNSSAI.SD = new(ngapType.SD)
	sliceSupportItem.SNSSAI.SD.Value = aper.OctetString("\x01\x02\x03")

	sliceSupportList.List = append(sliceSupportList.List, sliceSupportItem)

	broadcastPLMNList.List = append(broadcastPLMNList.List, broadcastPLMNItem)

	supportedTAList.List = append(supportedTAList.List, supportedTAItem)

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// PagingDRX
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
	ie.Value.DefaultPagingDRX = new(ngapType.PagingDRX)

	pagingDRX := ie.Value.DefaultPagingDRX
	pagingDRX.Value = ngapType.PagingDRXPresentV128
	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	return pdu
}
