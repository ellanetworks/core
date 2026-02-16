package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type DeregistrationRequestOpts struct {
	Guti *nasType.GUTI5G
	Suci *nasType.MobileIdentity5GS
	Ksi  int32
}

func BuildDeregistrationRequest(opts *DeregistrationRequestOpts) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)

	deregistrationRequest := nasMessage.NewDeregistrationRequestUEOriginatingDeregistration(0)

	deregistrationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	deregistrationRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	deregistrationRequest.SetSpareHalfOctet(0x00)
	deregistrationRequest.SetSwitchOff(1)
	deregistrationRequest.SetReRegistrationRequired(0)
	deregistrationRequest.SetAccessType(1)
	deregistrationRequest.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	deregistrationRequest.SetTSC(nasMessage.TypeOfSecurityContextFlagNative)

	deregistrationRequest.SetNasKeySetIdentifiler(uint8(opts.Ksi))
	// If AMF previously assigned the UE a 5G-GUTI, reuses it
	// If the 5G-GUTI is no longer valid, AMF will issue an Identity Request
	// which we'll answer with the requested Mobility Identity (eg. SUCI)
	if opts.Guti != nil {
		deregistrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{
			Iei:    opts.Guti.Iei,
			Len:    opts.Guti.Len,
			Buffer: opts.Guti.Octet[:],
		}
	} else {
		if opts.Suci == nil {
			return nil, fmt.Errorf("either Guti or Suci must be provided")
		}

		deregistrationRequest.MobileIdentity5GS = *opts.Suci
	}

	m.DeregistrationRequestUEOriginatingDeregistration = deregistrationRequest

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("error encoding gmm message: %v", err)
	}

	nasPdu := data.Bytes()

	return nasPdu, nil
}
