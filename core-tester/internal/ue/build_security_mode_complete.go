package ue

import (
	"bytes"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type SecurityModeCompleteOpts struct {
	UESecurity       *UESecurity
	IMEISV           string
	PDUSessionStatus *[16]bool
}

func BuildSecurityModeComplete(opts *SecurityModeCompleteOpts) ([]byte, error) {
	regReqOpts := &RegistrationRequestOpts{
		RegistrationType:  nasMessage.RegistrationType5GSInitialRegistration,
		RequestedNSSAI:    nil,
		UplinkDataStatus:  nil,
		IncludeCapability: true,
		UESecurity:        opts.UESecurity,
		PDUSessionStatus:  opts.PDUSessionStatus,
	}

	registrationRequest, err := BuildRegistrationRequest(regReqOpts)
	if err != nil {
		return nil, fmt.Errorf("error encoding %s IMSI UE  NAS Registration Request message: %v", opts.UESecurity.Supi, err)
	}

	pdu, err := buildSecurityModeComplete(registrationRequest, opts.IMEISV)
	if err != nil {
		return nil, fmt.Errorf("error encoding %s IMSI UE  NAS Security Mode Complete message: %v", opts.UESecurity.Supi, err)
	}

	return pdu, nil
}

func buildSecurityModeComplete(nasMessageContainer []uint8, imeiSV string) ([]byte, error) {
	imeisv, err := BuildIMEISV(imeiSV)
	if err != nil {
		return nil, fmt.Errorf("error building IMEISV: %v", err)
	}

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeSecurityModeComplete)

	securityModeComplete := nasMessage.NewSecurityModeComplete(0)
	securityModeComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	securityModeComplete.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	securityModeComplete.SetSpareHalfOctet(0)
	securityModeComplete.SetMessageType(nas.MsgTypeSecurityModeComplete)

	securityModeComplete.IMEISV = imeisv

	if nasMessageContainer != nil {
		securityModeComplete.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.SecurityModeCompleteNASMessageContainerType)
		securityModeComplete.NASMessageContainer.SetLen(uint16(len(nasMessageContainer)))
		securityModeComplete.SetNASMessageContainerContents(nasMessageContainer)
	}

	m.SecurityModeComplete = securityModeComplete

	data := new(bytes.Buffer)

	err = m.GmmMessageEncode(data)
	if err != nil {
		return nil, fmt.Errorf("error encoding IMSI UE  NAS Security Mode Complete message: %v", err)
	}

	nasPdu := data.Bytes()

	return nasPdu, nil
}

func BuildIMEISV(imeisv string) (*nasType.IMEISV, error) {
	if len(imeisv) != 16 {
		return nil, fmt.Errorf("IMEISV must be 16 digits, got %d", len(imeisv))
	}

	for i := range 16 {
		if imeisv[i] < '0' || imeisv[i] > '9' {
			return nil, fmt.Errorf("IMEISV contains non-digit characters")
		}
	}

	// digits d[0..15] -> 1..16
	var d [16]uint8
	for i := range 16 {
		d[i] = imeisv[i] - '0'
	}

	pei := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType) // IEI = 0x77
	pei.SetLen(9)

	// Octet[0]: bits7..4 = digit1, bit3 = OddEven (0 = even), bits2..0 = Type (5 = IMEISV)
	pei.SetIdentityDigit1(d[0])
	pei.SetOddEvenIdic(0) // even number of digits (16)
	pei.SetTypeOfIdentity(nasMessage.MobileIdentity5GSTypeImeisv)

	// Octet[1]..Octet[7]:
	// lower nibble = digit2,4,6,8,10,12,14
	// upper nibble = digit3,5,7,9,11,13,15
	pei.SetIdentityDigitP(d[1])   // Octet[1] low  = digit2
	pei.SetIdentityDigitP_1(d[2]) // Octet[1] high = digit3

	pei.SetIdentityDigitP_2(d[3]) // Octet[2] low  = digit4
	pei.SetIdentityDigitP_3(d[4]) // Octet[2] high = digit5

	pei.SetIdentityDigitP_4(d[5]) // Octet[3] low  = digit6
	pei.SetIdentityDigitP_5(d[6]) // Octet[3] high = digit7

	pei.SetIdentityDigitP_6(d[7]) // Octet[4] low  = digit8
	pei.SetIdentityDigitP_7(d[8]) // Octet[4] high = digit9

	pei.SetIdentityDigitP_8(d[9])  // Octet[5] low  = digit10
	pei.SetIdentityDigitP_9(d[10]) // Octet[5] high = digit11

	pei.SetIdentityDigitP_10(d[11]) // Octet[6] low  = digit12
	pei.SetIdentityDigitP_11(d[12]) // Octet[6] high = digit13

	pei.SetIdentityDigitP_12(d[13]) // Octet[7] low  = digit14
	pei.SetIdentityDigitP_13(d[14]) // Octet[7] high = digit15

	// Octet[8] (last): upper nibble = 0xF (filler), lower nibble = digit16
	pei.SetIdentityDigitP_14(d[15]) // Octet[8] low  = digit16
	pei.SetIdentityDigitP_15(0xF)   // Octet[8] high = filler 0xF  (IMPORTANT)

	return pei, nil
}
