// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestRegistrationResultBits(t *testing.T) {
	for _, tc := range []struct {
		msg  RegistrationAccept
		wire string
	}{
		{RegistrationAccept{RegistrationResult: RegistrationResult3GPP}, "7e00420101"},
		{RegistrationAccept{RegistrationResult: RegistrationResult3GPP, SMSAllowed: true}, "7e00420109"},
		{RegistrationAccept{RegistrationResult: RegistrationResult3GPPAndNon3GPP, SMSAllowed: true, NSSAAToBePerformed: true, EmergencyRegistered: true, DisasterRoamingResult: true}, "7e0042017b"},
	} {
		b, err := tc.msg.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(b) != tc.wire {
			t.Fatalf("REGISTRATION ACCEPT = %x, want %s", b, tc.wire)
		}

		got, err := ParseRegistrationAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if got.RegistrationResult != tc.msg.RegistrationResult || got.SMSAllowed != tc.msg.SMSAllowed ||
			got.NSSAAToBePerformed != tc.msg.NSSAAToBePerformed || got.EmergencyRegistered != tc.msg.EmergencyRegistered ||
			got.DisasterRoamingResult != tc.msg.DisasterRoamingResult {
			t.Fatalf("parsed %+v, want %+v", got, tc.msg)
		}
	}
}

func TestRegistrationResultSpareBitIgnored(t *testing.T) {
	got, err := ParseRegistrationAccept([]byte{0x7e, 0x00, 0x42, 0x01, 0x89})
	if err != nil {
		t.Fatal(err)
	}

	if got.RegistrationResult != RegistrationResult3GPP || !got.SMSAllowed {
		t.Fatalf("parsed %+v, want 3GPP access with SMS allowed", got)
	}
}

func TestConfigurationUpdateCommandSMSIndication(t *testing.T) {
	for _, available := range []bool{true, false} {
		m := &ConfigurationUpdateCommand{SMSAvailable: &available}

		b, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		want := []byte{0x7e, 0x00, 0x54, 0xf0}
		if available {
			want[3] = 0xf1
		}

		if !bytes.Equal(b, want) {
			t.Fatalf("CONFIGURATION UPDATE COMMAND = % x, want % x", b, want)
		}

		got, err := ParseConfigurationUpdateCommand(b)
		if err != nil {
			t.Fatal(err)
		}

		if got.SMSAvailable == nil || *got.SMSAvailable != available {
			t.Fatalf("SMS available = %v, want %t", got.SMSAvailable, available)
		}
	}
}
