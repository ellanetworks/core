// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func wire(t *testing.T, name string, fn func() ([]byte, error), want string) {
	t.Helper()

	if got := hex.EncodeToString(mustMarshal(t, fn)); got != want {
		t.Errorf("%s = %s, want %s", name, got, want)
	}
}

func TestMMBuildersWireBytes(t *testing.T) {
	rand := &[16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	autn := &[16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	t3502 := uint8(30)

	wire(t, "AuthenticationRequest", (&AuthenticationRequest{NgKSI: 1, ABBA: []byte{0, 0}, RAND: rand, AUTN: autn}).Marshal,
		"7e005601020000210102030405060708090a0b0c0d0e0f102010100f0e0d0c0b0a090807060504030201")
	wire(t, "AuthenticationReject", (&AuthenticationReject{}).Marshal, "7e0058")
	wire(t, "IdentityRequest", (&IdentityRequest{IdentityType: 1}).Marshal, "7e005b01")
	wire(t, "Status5GMM", (&Status5GMM{Cause: 0x6f}).Marshal, "7e00646f")
	wire(t, "ServiceReject", (&ServiceReject{Cause: 0x16}).Marshal, "7e004d16")
	wire(t, "RegistrationReject", (&RegistrationReject{Cause: 0x0b}).Marshal, "7e00440b")
	wire(t, "RegistrationRejectT3502", (&RegistrationReject{Cause: 0x0b, T3502: &t3502}).Marshal, "7e00440b16011e")
	wire(t, "DLNASTransport", (&DLNASTransport{PayloadContainerType: 1, PayloadContainer: []byte{0xAA, 0xBB, 0xCC}, PDUSessionID: 5}).Marshal,
		"7e0068010003aabbcc1205")

	imeisv := IMEISVRequested
	addInfo := uint8(0x02)
	wire(t, "SecurityModeCommand",
		(&SecurityModeCommand{CipheringAlgorithm: 2, IntegrityAlgorithm: 1, NgKSI: 1, ReplayedUESecCap: []byte{0xFF, 0xF0}, IMEISVRequest: &imeisv, Additional5GSecInfo: &addInfo}).Marshal,
		"7e005d210102fff0e1360102")

	wire(t, "DeregistrationRequestUETerminated", (&DeregistrationRequestUETerminated{AccessType: AccessType3GPP}).Marshal, "7e004701")
	wire(t, "DeregistrationAcceptUEOriginating", (&DeregistrationAcceptUEOriginating{}).Marshal, "7e0046")

	var psi [16]bool

	psi[1] = true
	wire(t, "ServiceAccept", (&ServiceAccept{PDUSessionStatus: PSIToBytes(psi)}).Marshal, "7e004e50020200")

	t3512 := EncodeGPRSTimer3(3240) // 5 * 10 minutes → 0x05
	wire(t, "RegistrationAccept",
		(&RegistrationAccept{RegistrationResult: AccessType3GPP, T3512Value: &t3512}).Marshal, "7e004201015e0105")

	ack := uint8(1)
	wire(t, "ConfigurationUpdateCommand", (&ConfigurationUpdateCommand{ConfigurationUpdateIndication: &ack}).Marshal, "7e0054d1")
}

func TestMMParsers(t *testing.T) {
	res := bytes.Repeat([]byte{0xAB}, 16)

	resp, err := ParseAuthenticationResponse(append([]byte{EPD5GMM, 0x00, uint8(MsgAuthenticationResponse), ieiAuthResponseParam, 0x10}, res...))
	if err != nil || !bytes.Equal(resp.RES, res) {
		t.Errorf("AuthenticationResponse RES = %x (err %v)", resp.RES, err)
	}

	auts := bytes.Repeat([]byte{0xCD}, 14)

	fail, err := ParseAuthenticationFailure(append([]byte{EPD5GMM, 0x00, uint8(MsgAuthenticationFailure), 0x15, ieiAuthFailureParam, 0x0e}, auts...))
	if err != nil || fail.Cause != 0x15 || !bytes.Equal(fail.AUTS, auts) {
		t.Errorf("AuthenticationFailure cause=%#x AUTS=%x (err %v)", fail.Cause, fail.AUTS, err)
	}

	mi := []byte{0x01, 0xAB, 0xCD}

	id, err := ParseIdentityResponse(append([]byte{EPD5GMM, 0x00, uint8(MsgIdentityResponse), 0x00, 0x03}, mi...))
	if err != nil || !bytes.Equal(id.MobileIdentity, mi) {
		t.Errorf("IdentityResponse MobileIdentity = %x (err %v)", id.MobileIdentity, err)
	}

	st, err := ParseStatus5GMM([]byte{EPD5GMM, 0x00, uint8(Msg5GMMStatus), 0x6f})
	if err != nil || st.Cause != 0x6f {
		t.Errorf("Status5GMM cause = %#x (err %v)", st.Cause, err)
	}

	if _, err := ParseSecurityModeReject([]byte{EPD5GMM, 0x00, uint8(MsgSecurityModeReject), 0x18}); err != nil {
		t.Errorf("SecurityModeReject: %v", err)
	}
}
