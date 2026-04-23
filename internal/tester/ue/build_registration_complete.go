package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type RegistrationCompleteOpts struct {
	SORTransparentContainer []uint8
}

func BuildRegistrationComplete(opts *RegistrationCompleteOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationCompleteOpts is nil")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationComplete)

	registrationComplete := nasMessage.NewRegistrationComplete(0)
	registrationComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationComplete.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationComplete.SetSpareHalfOctet(0)
	registrationComplete.SetMessageType(nas.MsgTypeRegistrationComplete)

	if opts.SORTransparentContainer != nil {
		registrationComplete.SORTransparentContainer = nasType.NewSORTransparentContainer(nasMessage.RegistrationCompleteSORTransparentContainerType)
		registrationComplete.SetLen(uint16(len(opts.SORTransparentContainer)))
		registrationComplete.SetSORContent(opts.SORTransparentContainer)
	}

	m.RegistrationComplete = registrationComplete

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GMM message: %v", err)
	}

	return data.Bytes(), nil
}
