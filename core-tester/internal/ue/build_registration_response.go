package ue

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type AuthenticationResponseOpts struct {
	AuthenticationResponseParam []uint8
	EapMsg                      string
}

func BuildAuthenticationResponse(opts *AuthenticationResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("AuthenticationResponseOpts is nil")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeAuthenticationResponse)

	authenticationResponse := nasMessage.NewAuthenticationResponse(0)
	authenticationResponse.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	authenticationResponse.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	authenticationResponse.SetSpareHalfOctet(0)
	authenticationResponse.SetMessageType(nas.MsgTypeAuthenticationResponse)

	if len(opts.AuthenticationResponseParam) > 0 {
		authenticationResponse.AuthenticationResponseParameter = nasType.NewAuthenticationResponseParameter(nasMessage.AuthenticationResponseAuthenticationResponseParameterType)
		authenticationResponse.AuthenticationResponseParameter.SetLen(uint8(len(opts.AuthenticationResponseParam)))
		copy(authenticationResponse.AuthenticationResponseParameter.Octet[:], opts.AuthenticationResponseParam[0:16])
	} else if opts.EapMsg != "" {
		rawEapMsg, err := base64.StdEncoding.DecodeString(opts.EapMsg)
		if err != nil {
			return nil, fmt.Errorf("could not decode eap msg: %v", err)
		}

		authenticationResponse.EAPMessage = nasType.NewEAPMessage(nasMessage.AuthenticationResponseEAPMessageType)
		authenticationResponse.EAPMessage.SetLen(uint16(len(rawEapMsg)))
		authenticationResponse.SetEAPMessage(rawEapMsg)
	}

	m.AuthenticationResponse = authenticationResponse

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode gmm message: %v", err)
	}

	nasPdu := data.Bytes()

	return nasPdu, nil
}
