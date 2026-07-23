// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	naslib "github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

// updateGolden regenerates the golden JSON fixtures: `go test ./internal/decoder/nas/
// -run TestDecoderGolden -update`. It locks the decoder's JSON output so the migration
// off free5gc can be proven byte-for-byte unchanged (spec_nas.md). The corpus is built
// with free5gc; the decoder under test is what the migration replaces, so this doubles
// as a free5gc↔fgs interop check.
var updateGolden = flag.Bool("update", false, "regenerate decoder golden JSON fixtures")

func encodeGmm(t *testing.T, mt uint8, populate func(*naslib.GmmMessage)) []byte {
	t.Helper()

	m := naslib.NewMessage()
	m.GmmMessage = naslib.NewGmmMessage()
	m.GmmHeader.SetMessageType(mt)
	populate(m.GmmMessage)

	var buf bytes.Buffer
	if err := m.GmmMessageEncode(&buf); err != nil {
		t.Fatalf("encode gmm 0x%02x: %v", mt, err)
	}

	return buf.Bytes()
}

func gmmPlainHdr() (nasType.ExtendedProtocolDiscriminator, nasType.SpareHalfOctetAndSecurityHeaderType) {
	var e nasType.ExtendedProtocolDiscriminator

	e.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)

	var s nasType.SpareHalfOctetAndSecurityHeaderType

	s.SetSecurityHeaderType(naslib.SecurityHeaderTypePlainNas)

	return e, s
}

// goldenCorpus builds the raw NAS PDUs the golden test renders: the security-header
// variants plus the plain 5GMM message types.
func goldenCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	// A known-good plain REGISTRATION REQUEST wire capture.
	regReq, err := decodeB64("fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A==")
	if err != nil {
		t.Fatalf("decode registration request: %v", err)
	}

	corpus := map[string][]byte{
		"registration_request":   regReq,
		"secprot_integrity_only": append([]byte{0x7e, 0x01, 0xde, 0xad, 0xbe, 0xef, 0x05}, regReq...),
		"secprot_ciphered_nea0":  append([]byte{0x7e, 0x02, 0xde, 0xad, 0xbe, 0xef, 0x05}, regReq...),
		"too_short":              {0x7e, 0x00},
		"too_short_1byte":        {0x7e}, // guarded; would previously panic reading raw[1]
		"empty":                  {},
		"unsupported_sht":        append([]byte{0x7e, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00}, regReq...),
	}

	corpus["identity_request"] = encodeGmm(t, naslib.MsgTypeIdentityRequest, func(g *naslib.GmmMessage) {
		m := nasMessage.NewIdentityRequest(0)
		m.ExtendedProtocolDiscriminator, m.SpareHalfOctetAndSecurityHeaderType = gmmPlainHdr()
		m.SetMessageType(naslib.MsgTypeIdentityRequest)
		m.SetTypeOfIdentity(nasMessage.MobileIdentity5GSTypeSuci)
		g.IdentityRequest = m
	})

	corpus["authentication_reject"] = encodeGmm(t, naslib.MsgTypeAuthenticationReject, func(g *naslib.GmmMessage) {
		m := nasMessage.NewAuthenticationReject(0)
		m.ExtendedProtocolDiscriminator, m.SpareHalfOctetAndSecurityHeaderType = gmmPlainHdr()
		m.SetMessageType(naslib.MsgTypeAuthenticationReject)
		g.AuthenticationReject = m
	})

	corpus["registration_reject"] = encodeGmm(t, naslib.MsgTypeRegistrationReject, func(g *naslib.GmmMessage) {
		m := nasMessage.NewRegistrationReject(0)
		m.ExtendedProtocolDiscriminator, m.SpareHalfOctetAndSecurityHeaderType = gmmPlainHdr()
		m.SetMessageType(naslib.MsgTypeRegistrationReject)
		m.SetCauseValue(nasMessage.Cause5GMMPLMNNotAllowed)
		g.RegistrationReject = m
	})

	corpus["service_reject"] = encodeGmm(t, naslib.MsgTypeServiceReject, func(g *naslib.GmmMessage) {
		m := nasMessage.NewServiceReject(0)
		m.ExtendedProtocolDiscriminator, m.SpareHalfOctetAndSecurityHeaderType = gmmPlainHdr()
		m.SetMessageType(naslib.MsgTypeServiceReject)
		m.SetCauseValue(nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		g.ServiceReject = m
	})

	corpus["registration_complete"] = encodeGmm(t, naslib.MsgTypeRegistrationComplete, func(g *naslib.GmmMessage) {
		m := nasMessage.NewRegistrationComplete(0)
		m.ExtendedProtocolDiscriminator, m.SpareHalfOctetAndSecurityHeaderType = gmmPlainHdr()
		m.SetMessageType(naslib.MsgTypeRegistrationComplete)
		g.RegistrationComplete = m
	})

	return corpus
}

func TestDecoderGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")

	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
	}

	corpus := goldenCorpus(t)

	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(nas.DecodeNASMessage(corpus[name]), "", "  ")
			if err != nil {
				t.Fatalf("marshal decoded JSON: %v", err)
			}

			got = append(got, '\n')
			path := filepath.Join(dir, name+".json")

			if *updateGolden {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden %q: %v", path, err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %q (run with -update to create): %v", path, err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("decoder JSON changed for %q.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}
