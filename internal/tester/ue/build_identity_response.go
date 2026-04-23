package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type IdentityResponseOpts struct {
	Suci nasType.MobileIdentity5GS
}

func BuildIdentityResponse(opts *IdentityResponseOpts) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeIdentityResponse)

	identityResponse := nasMessage.NewIdentityResponse(0)
	identityResponse.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	identityResponse.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	identityResponse.SetSpareHalfOctet(0x00)
	identityResponse.SetMessageType(nas.MsgTypeIdentityResponse)
	identityResponse.MobileIdentity = nasType.MobileIdentity(opts.Suci)

	m.IdentityResponse = identityResponse

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode GMM message: %v", err)
	}

	nasPdu := data.Bytes()

	return nasPdu, nil
}
