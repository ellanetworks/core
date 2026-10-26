// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ellanetworks/core/diameter/tgpp"
)

var ErrInvalidUserData = errors.New("sh: invalid user data")

type ShData struct {
	SMSRegistration *SMSRegistrationInfo
}

type SMSRegistrationInfo struct {
	IPSMGWNumber string
	SCAddress    string
	SBISupported *bool
}

type xmlShData struct {
	XMLName         xml.Name                `xml:"Sh-Data"`
	SMSRegistration *xmlSMSRegistrationInfo `xml:"Sh-IMS-Data>Extension>Extension>Extension>SMSRegistrationInfo"`
}

type xmlSMSRegistrationInfo struct {
	IPSMGWNumber *string                      `xml:"IP-SM-GW-Number"`
	Extension    *xmlSMSRegistrationExtension `xml:"Extension"`
}

type xmlSMSRegistrationExtension struct {
	SCAddress string                        `xml:"SCAddress,omitempty"`
	Extension *xmlSMSRegistrationExtension2 `xml:"Extension"`
}

type xmlSMSRegistrationExtension2 struct {
	SBISupportIndication *string `xml:"IP-SM-GW-SBISupportIndication"`
}

func MarshalShData(d ShData) ([]byte, error) {
	if err := d.validate(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	buf.WriteString(xml.Header)

	if err := xml.NewEncoder(&buf).Encode(d.wire()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
	}

	return buf.Bytes(), nil
}

func ParseShData(b []byte) (ShData, error) {
	var w xmlShData

	dec := xml.NewDecoder(bytes.NewReader(b))

	if err := dec.Decode(&w); err != nil {
		return ShData{}, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
	}

	if err := requireEnd(dec); err != nil {
		return ShData{}, err
	}

	d, err := w.model()
	if err != nil {
		return ShData{}, err
	}

	if err := d.validate(); err != nil {
		return ShData{}, err
	}

	return d, nil
}

func requireEnd(d *xml.Decoder) error {
	for {
		tok, err := d.Token()

		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("%w: %w", ErrInvalidUserData, err)
		}

		switch t := tok.(type) {
		case xml.Comment, xml.ProcInst:
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return userDataError("content after Sh-Data")
			}
		default:
			return userDataError("content after Sh-Data")
		}
	}
}

func userDataError(format string, args ...any) error {
	return fmt.Errorf("%w: %w", ErrInvalidUserData, fmt.Errorf(format, args...))
}

func (d ShData) validate() error {
	if d.SMSRegistration == nil {
		return nil
	}

	r := d.SMSRegistration

	if r.IPSMGWNumber != "" && !tgpp.ValidE164(r.IPSMGWNumber) {
		return userDataError("IP-SM-GW number %q", r.IPSMGWNumber)
	}

	if r.SCAddress != "" && !tgpp.ValidE164(r.SCAddress) {
		return userDataError("SC address %q", r.SCAddress)
	}

	return nil
}

func (d ShData) wire() xmlShData {
	var w xmlShData

	if r := d.SMSRegistration; r != nil {
		number := r.IPSMGWNumber
		w.SMSRegistration = &xmlSMSRegistrationInfo{IPSMGWNumber: &number}

		if r.SCAddress != "" || r.SBISupported != nil {
			ext := &xmlSMSRegistrationExtension{SCAddress: r.SCAddress}

			if r.SBISupported != nil {
				indication := "0"
				if *r.SBISupported {
					indication = "1"
				}

				ext.Extension = &xmlSMSRegistrationExtension2{SBISupportIndication: &indication}
			}

			w.SMSRegistration.Extension = ext
		}
	}

	return w
}

func (w xmlShData) model() (ShData, error) {
	var d ShData

	r := w.SMSRegistration
	if r == nil {
		return d, nil
	}

	if r.IPSMGWNumber == nil {
		return ShData{}, userDataError("SMSRegistrationInfo without an IP-SM-GW-Number")
	}

	info := &SMSRegistrationInfo{IPSMGWNumber: strings.TrimSpace(*r.IPSMGWNumber)}

	if ext := r.Extension; ext != nil {
		info.SCAddress = strings.TrimSpace(ext.SCAddress)

		if ext.Extension != nil && ext.Extension.SBISupportIndication != nil {
			switch strings.TrimSpace(*ext.Extension.SBISupportIndication) {
			case "0":
				info.SBISupported = new(bool)
			case "1":
				supported := true
				info.SBISupported = &supported
			default:
				return ShData{}, userDataError("IP-SM-GW-SBISupportIndication %q", *ext.Extension.SBISupportIndication)
			}
		}
	}

	d.SMSRegistration = info

	return d, nil
}
