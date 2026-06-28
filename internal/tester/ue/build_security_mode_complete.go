// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	var d [16]uint8
	for i := range 16 {
		d[i] = imeisv[i] - '0'
	}

	pei := nasType.NewIMEISV(nasMessage.SecurityModeCompleteIMEISVType)
	pei.SetLen(9)

	pei.SetIdentityDigit1(d[0])
	pei.SetOddEvenIdic(0)
	pei.SetTypeOfIdentity(nasMessage.MobileIdentity5GSTypeImeisv)

	// TS 24.501 §9.11.3.4: each octet packs the next even digit in the low nibble
	// and the next odd digit in the high nibble; the unused final high nibble is
	// filled with 0xF.
	pei.SetIdentityDigitP(d[1])
	pei.SetIdentityDigitP_1(d[2])
	pei.SetIdentityDigitP_2(d[3])
	pei.SetIdentityDigitP_3(d[4])
	pei.SetIdentityDigitP_4(d[5])
	pei.SetIdentityDigitP_5(d[6])
	pei.SetIdentityDigitP_6(d[7])
	pei.SetIdentityDigitP_7(d[8])
	pei.SetIdentityDigitP_8(d[9])
	pei.SetIdentityDigitP_9(d[10])
	pei.SetIdentityDigitP_10(d[11])
	pei.SetIdentityDigitP_11(d[12])
	pei.SetIdentityDigitP_12(d[13])
	pei.SetIdentityDigitP_13(d[14])
	pei.SetIdentityDigitP_14(d[15])
	pei.SetIdentityDigitP_15(0xF)

	return pei, nil
}
