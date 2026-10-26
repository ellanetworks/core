// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
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
	dec := xml.NewDecoder(bytes.NewReader(b))

	root, err := rootElement(dec)
	if err != nil {
		return ShData{}, err
	}

	if root.Name.Local != "Sh-Data" {
		return ShData{}, userDataError("root element %s, want Sh-Data", root.Name.Local)
	}

	var d ShData

	err = descend(dec, root, root.Name.Space, smsRegistrationPath, func(el xml.StartElement) error {
		d.SMSRegistration, err = decodeSMSRegistration(dec, el, root.Name.Space)
		return err
	})
	if err != nil {
		return ShData{}, err
	}

	if err := requireEnd(dec); err != nil {
		return ShData{}, err
	}

	if err := d.validate(); err != nil {
		return ShData{}, err
	}

	return d, nil
}

var smsRegistrationPath = []string{"Sh-IMS-Data", "Extension", "Extension", "Extension", "SMSRegistrationInfo"}

func rootElement(d *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := d.Token()
		if err != nil {
			return xml.StartElement{}, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			return t, nil
		case xml.Comment, xml.ProcInst, xml.Directive:
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return xml.StartElement{}, userDataError("content before Sh-Data")
			}
		default:
			return xml.StartElement{}, userDataError("content before Sh-Data")
		}
	}
}

func descend(d *xml.Decoder, parent xml.StartElement, ns string, path []string, leaf func(xml.StartElement) error) error {
	return children(d, parent, ns, map[string]func(xml.StartElement) error{
		path[0]: func(el xml.StartElement) error {
			if len(path) == 1 {
				return leaf(el)
			}

			return descend(d, el, ns, path[1:], leaf)
		},
	})
}

func children(d *xml.Decoder, parent xml.StartElement, ns string, handlers map[string]func(xml.StartElement) error) error {
	seen := make(map[string]bool, len(handlers))

	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidUserData, err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			handle, known := handlers[t.Name.Local]
			if !known || t.Name.Space != ns {
				if err := d.Skip(); err != nil {
					return fmt.Errorf("%w: %w", ErrInvalidUserData, err)
				}

				continue
			}

			if seen[t.Name.Local] {
				return userDataError("more than one %s in %s", t.Name.Local, parent.Name.Local)
			}

			seen[t.Name.Local] = true

			if err := handle(t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func text(d *xml.Decoder, el xml.StartElement) (string, error) {
	var buf bytes.Buffer

	for {
		tok, err := d.Token()
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalidUserData, err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.StartElement:
			return "", userDataError("element %s inside %s", t.Name.Local, el.Name.Local)
		case xml.EndElement:
			return strings.Trim(buf.String(), xmlWhitespace), nil
		}
	}
}

const xmlWhitespace = " \t\r\n"

func decodeSMSRegistration(d *xml.Decoder, el xml.StartElement, ns string) (*SMSRegistrationInfo, error) {
	var (
		info   SMSRegistrationInfo
		number *string
	)

	value := func(dst *string) func(xml.StartElement) error {
		return func(el xml.StartElement) error {
			v, err := text(d, el)
			*dst = v

			return err
		}
	}

	var indication *string

	err := children(d, el, ns, map[string]func(xml.StartElement) error{
		"IP-SM-GW-Number": func(el xml.StartElement) error {
			number = new(string)
			return value(number)(el)
		},
		"Extension": func(el xml.StartElement) error {
			return children(d, el, ns, map[string]func(xml.StartElement) error{
				"SCAddress": value(&info.SCAddress),
				"Extension": func(el xml.StartElement) error {
					return children(d, el, ns, map[string]func(xml.StartElement) error{
						"IP-SM-GW-SBISupportIndication": func(el xml.StartElement) error {
							indication = new(string)
							return value(indication)(el)
						},
					})
				},
			})
		},
	})
	if err != nil {
		return nil, err
	}

	if number == nil {
		return nil, userDataError("SMSRegistrationInfo without an IP-SM-GW-Number")
	}

	info.IPSMGWNumber = *number

	if indication != nil {
		if info.SBISupported, err = parseSBISupportIndication(*indication); err != nil {
			return nil, err
		}
	}

	return &info, nil
}

func parseSBISupportIndication(v string) (*bool, error) {
	n, err := strconv.ParseUint(strings.TrimPrefix(v, "+"), 10, 8)
	if err != nil || n > 1 {
		return nil, userDataError("IP-SM-GW-SBISupportIndication %q", v)
	}

	supported := n == 1

	return &supported, nil
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
