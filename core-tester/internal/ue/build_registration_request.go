package ue

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

type RegistrationRequestOpts struct {
	RegistrationType  uint8
	RequestedNSSAI    *nasType.RequestedNSSAI
	UplinkDataStatus  *nasType.UplinkDataStatus
	IncludeCapability bool
	UESecurity        *UESecurity
	PDUSessionStatus  *[16]bool
}

func BuildRegistrationRequest(opts *RegistrationRequestOpts) ([]byte, error) {
	if opts == nil {
		return nil, fmt.Errorf("RegistrationRequestOpts is nil")
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)

	registrationRequest := nasMessage.NewRegistrationRequest(0)
	registrationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	registrationRequest.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	registrationRequest.SetSpareHalfOctet(0x00)
	registrationRequest.SetMessageType(nas.MsgTypeRegistrationRequest)
	registrationRequest.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(uint8(opts.UESecurity.NgKsi.Ksi))
	registrationRequest.SetRegistrationType5GS(opts.RegistrationType)
	// If AMF previously assigned the UE a 5G-GUTI, reuses it
	// If the 5G-GUTI is no longer valid, AMF will issue an Identity Request
	// which we'll answer with the requested Mobility Identity (eg. SUCI)
	if opts.UESecurity.Guti != nil {
		guti := opts.UESecurity.Guti
		registrationRequest.MobileIdentity5GS = nasType.MobileIdentity5GS{
			Iei:    guti.Iei,
			Len:    guti.Len,
			Buffer: guti.Octet[:],
		}
	} else {
		registrationRequest.MobileIdentity5GS = opts.UESecurity.Suci
	}

	if opts.IncludeCapability {
		registrationRequest.Capability5GMM = &nasType.Capability5GMM{
			Iei:   nasMessage.RegistrationRequestCapability5GMMType,
			Len:   1,
			Octet: [13]uint8{0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		}
	} else {
		registrationRequest.Capability5GMM = nil
	}

	registrationRequest.UESecurityCapability = opts.UESecurity.UeSecurityCapability
	registrationRequest.RequestedNSSAI = opts.RequestedNSSAI
	registrationRequest.SetFOR(1)

	pduFlag := uint16(0)

	if opts.PDUSessionStatus != nil {
		for i, pduSession := range opts.PDUSessionStatus {
			pduFlag = pduFlag + (boolToUint16(pduSession) << (i))
		}

		if pduFlag != 0 {
			registrationRequest.UplinkDataStatus = new(nasType.UplinkDataStatus)
			registrationRequest.UplinkDataStatus.SetIei(nasMessage.RegistrationRequestUplinkDataStatusType)
			registrationRequest.UplinkDataStatus.SetLen(2)

			registrationRequest.UplinkDataStatus.Buffer = make([]byte, 2)
			binary.LittleEndian.PutUint16(registrationRequest.UplinkDataStatus.Buffer, pduFlag)

			registrationRequest.PDUSessionStatus = new(nasType.PDUSessionStatus)
			registrationRequest.PDUSessionStatus.SetIei(nasMessage.RegistrationRequestPDUSessionStatusType)
			registrationRequest.PDUSessionStatus.SetLen(2)
			registrationRequest.PDUSessionStatus.Buffer = registrationRequest.UplinkDataStatus.Buffer
		}
	}

	m.RegistrationRequest = registrationRequest

	data := new(bytes.Buffer)

	err := m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("error encoding GMM message: %w", err)
	}

	nasPdu := data.Bytes()

	if pduFlag != 0 {
		if err = security.NASEncrypt(opts.UESecurity.CipheringAlg, opts.UESecurity.KnasEnc, opts.UESecurity.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, nasPdu); err != nil {
			return nasPdu, fmt.Errorf("error encrypting NAS message: %w", err)
		}

		registrationRequest.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.RegistrationRequestNASMessageContainerType)
		registrationRequest.NASMessageContainer.SetLen(uint16(len(nasPdu)))
		registrationRequest.NASMessageContainer.Buffer = nasPdu

		registrationRequest.UplinkDataStatus = nil
		registrationRequest.PDUSessionStatus = nil

		data = new(bytes.Buffer)

		err = m.GmmMessageEncode(data)
		if err != nil {
			return nil, fmt.Errorf("error encoding GMM message: %w", err)
		}

		nasPdu = data.Bytes()
	}

	return nasPdu, nil
}

func boolToUint16(b bool) uint16 {
	if b {
		return 1
	}

	return 0
}
