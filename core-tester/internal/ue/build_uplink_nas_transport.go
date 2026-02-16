package ue

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/openapi/models"
)

type UplinkNasTransportOpts struct {
	PDUSessionID     uint8
	PayloadContainer []byte
	DNN              string
	SNSSAI           models.Snssai
}

func BuildUplinkNasTransport(opts *UplinkNasTransportOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("UplinkNasTransportOpts is nil")
	}

	if opts.PDUSessionID == 0 {
		return nil, fmt.Errorf("PDUSessionID is required to build UplinkNasTransport for PDU Session")
	}

	if opts.PayloadContainer == nil {
		return nil, fmt.Errorf("PayloadContainer is required to build UplinkNasTransport for PDU Session")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeULNASTransport)

	ulNasTransport := nasMessage.NewULNASTransport(0)
	ulNasTransport.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	ulNasTransport.SetMessageType(nas.MsgTypeULNASTransport)
	ulNasTransport.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	ulNasTransport.PduSessionID2Value = new(nasType.PduSessionID2Value)
	ulNasTransport.PduSessionID2Value.SetIei(nasMessage.ULNASTransportPduSessionID2ValueType)
	ulNasTransport.SetPduSessionID2Value(opts.PDUSessionID)
	ulNasTransport.RequestType = new(nasType.RequestType)
	ulNasTransport.RequestType.SetIei(nasMessage.ULNASTransportRequestTypeType)
	ulNasTransport.SetRequestTypeValue(nasMessage.ULNASTransportRequestTypeInitialRequest)

	if opts.DNN != "" {
		ulNasTransport.DNN = new(nasType.DNN)
		ulNasTransport.DNN.SetIei(nasMessage.ULNASTransportDNNType)
		ulNasTransport.DNN.SetLen(uint8(len(opts.DNN)))
		ulNasTransport.SetDNN(opts.DNN)
	}

	ulNasTransport.SNSSAI = nasType.NewSNSSAI(nasMessage.ULNASTransportSNSSAIType)
	if opts.SNSSAI.Sd == "" {
		ulNasTransport.SNSSAI.SetLen(1)
	} else {
		ulNasTransport.SNSSAI.SetLen(4)

		var sdTemp [3]uint8

		sd, err := hex.DecodeString(opts.SNSSAI.Sd)
		if err != nil {
			return nil, fmt.Errorf("failed to decode SD string: %v", err)
		}

		copy(sdTemp[:], sd)

		ulNasTransport.SetSD(sdTemp)
	}

	ulNasTransport.SetSST(uint8(opts.SNSSAI.Sst))

	ulNasTransport.SetPayloadContainerType(nasMessage.PayloadContainerTypeN1SMInfo)
	ulNasTransport.PayloadContainer.SetLen(uint16(len(opts.PayloadContainer)))
	ulNasTransport.SetPayloadContainerContents(opts.PayloadContainer)

	m.ULNASTransport = ulNasTransport

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GMM message: %v", err)
	}

	nasPdu := data.Bytes()

	return nasPdu, nil
}
