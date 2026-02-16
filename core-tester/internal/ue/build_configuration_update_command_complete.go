package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func BuildConfigurationUpdateComplete() ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	commandComplete := nasMessage.NewConfigurationUpdateComplete(0)
	commandComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	commandComplete.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	commandComplete.SetSpareHalfOctet(0)
	commandComplete.SetMessageType(nas.MsgTypeConfigurationUpdateComplete)

	m.ConfigurationUpdateComplete = commandComplete

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GMM message: %v", err)
	}

	return data.Bytes(), nil
}
