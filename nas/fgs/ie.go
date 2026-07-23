// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MobileIdentityType is the type-of-identity field (bits 1-3 of octet 1) of a 5GS
// mobile identity (TS 24.501 §9.11.3.4).
type MobileIdentityType uint8

const (
	IdentityNoIdentity MobileIdentityType = 0
	IdentitySUCI       MobileIdentityType = 1
	IdentityGUTI       MobileIdentityType = 2
	IdentityIMEI       MobileIdentityType = 3
	IdentitySTMSI      MobileIdentityType = 4
	IdentityIMEISV     MobileIdentityType = 5
)

// supiFormatNAI is the NAI SUPI format in the SUCI header (TS 24.501 §9.11.3.4,
// octet 1 bits 5-7); the default (0) is the IMSI SUPI format.
const supiFormatNAI uint8 = 1

// protectionSchemeNull is the null SUCI protection scheme, under which the scheme
// output is the cleartext MSIN (TS 24.501 §9.11.3.4, TS 33.501 Annex C).
const protectionSchemeNull = 0

// TypeOfIdentity returns the type of identity carried in the first octet of a 5GS
// mobile identity IE value (TS 24.501 §9.11.3.4).
func TypeOfIdentity(octet uint8) MobileIdentityType {
	return MobileIdentityType(octet & 0x07)
}

// SUCIToString decodes a SUCI mobile identity into its canonical string form and
// the PLMN identity (TS 24.501 §9.11.3.4, TS 23.003 §2.2B). For the IMSI SUPI
// format the string is "suci-0-<mcc>-<mnc>-<routing>-<scheme>-<hnpki>-<output>";
// for the NAI SUPI format it is "nai-1-<hex>" and the PLMN is empty.
func SUCIToString(buf []byte) (suci, plmnID string, err error) {
	if len(buf) < 1 {
		return "", "", errors.New("nas/fgs: SUCI too short")
	}

	if (buf[0]&0xf0)>>4 == supiFormatNAI {
		nai, nerr := naiToString(buf)
		return nai, "", nerr
	}

	if len(buf) < 9 {
		return "", "", errors.New("nas/fgs: SUCI too short")
	}

	mccDigit3 := buf[2] & 0x0f
	mcc := hex.EncodeToString([]byte{swapNibbles(buf[1]), mccDigit3 << 4})[:3]

	mncDigit3 := (buf[2] & 0xf0) >> 4

	mnc := hex.EncodeToString([]byte{swapNibbles(buf[3]), mncDigit3 << 4})
	if mnc[2] == 'f' {
		mnc = mnc[:2] // 2-digit MNC: drop the filler
	} else {
		mnc = mnc[:3]
	}

	plmnID = mcc + mnc

	routingInd := hex.EncodeToString([]byte{swapNibbles(buf[4]), swapNibbles(buf[5])})
	if idx := strings.Index(routingInd, "f"); idx != -1 {
		routingInd = routingInd[:idx]
	}

	protectionScheme := fmt.Sprintf("%x", buf[6])
	hnpki := fmt.Sprintf("%d", buf[7])

	var schemeOutput string

	if protectionScheme == strconv.Itoa(protectionSchemeNull) {
		var msin []byte
		for i := 8; i < len(buf); i++ {
			msin = append(msin, swapNibbles(buf[i]))
		}

		schemeOutput = hex.EncodeToString(msin)
		if schemeOutput[len(schemeOutput)-1] == 'f' {
			schemeOutput = schemeOutput[:len(schemeOutput)-1]
		}
	} else {
		schemeOutput = hex.EncodeToString(buf[8:])
	}

	suci = strings.Join([]string{"suci", "0", mcc, mnc, routingInd, protectionScheme, hnpki, schemeOutput}, "-")

	return suci, plmnID, nil
}

func naiToString(buf []byte) (string, error) {
	if len(buf) < 2 {
		return "", errors.New("nas/fgs: NAI too short")
	}

	return strings.Join([]string{"nai", "1", hex.EncodeToString(buf[1:])}, "-"), nil
}

// PEIToString decodes an IMEI (type 3) or IMEISV (type 5) mobile identity into its
// "imei-<15 digits>" / "imeisv-<16 digits>" string form (TS 24.501 §9.11.3.4,
// TS 23.003). The IMEI is rejected unless its Luhn check digit is valid.
func PEIToString(buf []byte) (string, error) {
	if len(buf) < 1 {
		return "", errors.New("nas/fgs: PEI too short")
	}

	prefix := "imeisv-"
	if TypeOfIdentity(buf[0]) == IdentityIMEI {
		prefix = "imei-"
	}

	oddIndication := (buf[0] & 0x08) >> 3

	// Digits are packed low-then-high nibble across octets; the identity-type/odd
	// nibble of octet 1 is a filler dropped below.
	digitBytes := []byte{buf[0] & 0xf0}
	for _, octet := range buf[1:] {
		digitBytes[len(digitBytes)-1] += octet & 0x0f
		digitBytes = append(digitBytes, octet&0xf0)
	}

	digits := hex.EncodeToString(digitBytes)
	digits = digits[:len(digits)-1] // drop the trailing filler nibble

	if oddIndication == 0 {
		digits = digits[:len(digits)-1]
	}

	if prefix == "imei-" {
		if len(digits) != 15 {
			return "", fmt.Errorf("nas/fgs: invalid IMEI length %d", len(digits))
		}

		valid, verr := validateIMEI(digits)
		if verr != nil {
			return "", verr
		}

		if !valid {
			return "", errors.New("nas/fgs: invalid IMEI checksum")
		}
	} else if len(digits) != 16 {
		return "", fmt.Errorf("nas/fgs: invalid IMEISV length %d", len(digits))
	}

	return prefix + digits, nil
}

// swapNibbles exchanges the high and low nibble of b, undoing the swapped-nibble
// BCD packing of 3GPP identities (TS 23.003).
func swapNibbles(b byte) byte {
	return b<<4 | b>>4
}

// validateIMEI reports whether the 15-digit IMEI string passes the Luhn checksum
// (TS 23.003 §6.2.1). It errors on any non-decimal character.
func validateIMEI(imei string) (bool, error) {
	for _, c := range imei {
		if c < '0' || c > '9' {
			return false, fmt.Errorf("nas/fgs: IMEI contains non-digit character: %c", c)
		}
	}

	sum := 0

	for i := len(imei) - 1; i >= 0; i-- {
		digit := int(imei[i] - '0')

		if (len(imei)-i)%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit = digit/10 + digit%10
			}
		}

		sum += digit
	}

	return sum%10 == 0, nil
}
