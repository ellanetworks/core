// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"maps"
	"slices"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/prometheus/client_golang/prometheus"
)

var pduSessionEstablishmentRequest = []byte{0x2E, 0x05, 0x01, 0xC1, 0xFF, 0xFF}

func dlNASTransport(t *testing.T, pct fgs.PayloadContainerType, payload []byte, cause *fgs.GMMCause) []byte {
	t.Helper()

	b, err := (&fgs.DLNASTransport{
		PayloadContainerType: pct,
		PayloadContainer:     payload,
		PDUSessionID:         new(fgs.PDUSessionID(5)),
		Cause:                cause,
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal DL NAS transport: %v", err)
	}

	return b
}

func TestNASTypes5G(t *testing.T) {
	notForwarded := fgs.GMMCausePayloadNotForwarded
	acceptSM := []byte{0x2E, 0x05, 0x01, 0xC2}

	ulTransport := append([]byte{0x7E, 0x00, 0x67, 0x01, 0x00, byte(len(pduSessionEstablishmentRequest))}, pduSessionEstablishmentRequest...)
	ulSMS := append([]byte{0x7E, 0x00, 0x67, 0x02, 0x00, 0x03}, 0x09, 0x01, 0x00)

	cases := []struct {
		name string
		msg  []byte
		want []string
	}{
		{"registration request", []byte{0x7E, 0x00, 0x41, 0x79}, []string{"RegistrationRequest"}},
		{"service reject", []byte{0x7E, 0x00, 0x4D, 0x09}, []string{"ServiceReject"}},
		{"5gmm status", []byte{0x7E, 0x00, 0x64, 0x6F}, []string{"5GMMStatus"}},
		{"deregistration request UE originating", []byte{0x7E, 0x00, 0x45, 0x01}, []string{"DeregistrationRequestUEOriginating"}},
		{"unassigned 5gmm type", []byte{0x7E, 0x00, 0x40}, []string{NASTypeUnknown}},
		{"security protected", []byte{0x7E, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, 0x7E, 0x00, 0x41}, []string{NASTypeUnknown}},
		{"standalone 5gsm", []byte{0x2E, 0x05, 0x01, 0xCA}, []string{"PDUSessionModificationReject"}},
		{"unassigned 5gsm type", []byte{0x2E, 0x05, 0x01, 0xC0}, []string{NASTypeUnknown}},
		{"unknown epd", []byte{0x0F, 0x00, 0x41}, []string{NASTypeUnknown}},
		{"too short", []byte{0x7E, 0x00}, []string{NASTypeUnknown}},
		{"empty", nil, []string{NASTypeUnknown}},
		{"ul nas transport with n1 sm", ulTransport, []string{"ULNASTransport", "PDUSessionEstablishmentRequest"}},
		{"ul nas transport with sms", ulSMS, []string{"ULNASTransport"}},
		{"ul nas transport truncated payload", ulTransport[:7], []string{"ULNASTransport"}},
		{"dl nas transport with n1 sm", dlNASTransport(t, fgs.PayloadContainerTypeN1SMInfo, acceptSM, nil), []string{"DLNASTransport", "PDUSessionEstablishmentAccept"}},
		{"dl nas transport returning an unforwarded payload", dlNASTransport(t, fgs.PayloadContainerTypeN1SMInfo, pduSessionEstablishmentRequest, &notForwarded), []string{"DLNASTransport"}},
		{"dl nas transport with sms", dlNASTransport(t, fgs.PayloadContainerTypeSMS, []byte{0x09, 0x01, 0x00}, nil), []string{"DLNASTransport"}},
		{"dl nas transport with undecodable n1 sm", dlNASTransport(t, fgs.PayloadContainerTypeN1SMInfo, []byte{0x2E, 0x05}, nil), []string{"DLNASTransport", NASTypeUnknown}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nasTypes5G(tc.msg); !slices.Equal(got, tc.want) {
				t.Errorf("nasTypes5G = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadable5GS(t *testing.T) {
	inner := []byte{0x7E, 0x00, 0x4C, 0x10}
	header := func(sht byte) []byte { return []byte{0x7E, sht, 0xAA, 0xBB, 0xCC, 0xDD, 0x03} }

	cases := []struct {
		name string
		pdu  []byte
		want []string
	}{
		{"plain", inner, []string{"ServiceRequest"}},
		{"integrity protected", append(header(0x01), inner...), []string{"ServiceRequest"}},
		{"integrity protected new context", append(header(0x03), 0x7E, 0x00, 0x5D), []string{"SecurityModeCommand"}},
		{"ciphered", append(header(0x02), inner...), []string{NASTypeUnknown}},
		{"ciphered new context", append(header(0x04), inner...), []string{NASTypeUnknown}},
		{"integrity protected without body", header(0x01), []string{NASTypeUnknown}},
		{"standalone 5gsm", pduSessionEstablishmentRequest, []string{"PDUSessionEstablishmentRequest"}},
		{"empty", nil, []string{NASTypeUnknown}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nasTypes5G(readable5GS(tc.pdu)); !slices.Equal(got, tc.want) {
				t.Errorf("types = %v, want %v", got, tc.want)
			}
		})
	}
}

func marshal(t *testing.T, m interface{ MarshalBinary() ([]byte, error) }) []byte {
	t.Helper()

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return b
}

func TestNASTypes4G(t *testing.T) {
	pdnConnectivityRequest := []byte{0x02, 0x01, 0xD0, 0x11}
	defaultBearerRequest := []byte{0x52, 0x00, 0xC1}
	defaultBearerAccept := []byte{0x52, 0x00, 0xC2}
	pdnConnectivityReject := []byte{0x02, 0x01, 0xD1, 0x1B}

	attachRequest := []byte{0x07, 0x41, 0x71, 0x02, 0xAA, 0xBB, 0x02, 0xE0, 0xE0, 0x00, byte(len(pdnConnectivityRequest))}
	attachRequest = append(attachRequest, pdnConnectivityRequest...)

	attachAccept := []byte{0x07, 0x42, 0x01, 0x21, 0x03, 0x00, 0x00, 0x01, 0x00, byte(len(defaultBearerRequest))}
	attachAccept = append(attachAccept, defaultBearerRequest...)

	cases := []struct {
		name string
		msg  []byte
		want []string
	}{
		{"attach request with pdn connectivity request", attachRequest, []string{"AttachRequest", "PDNConnectivityRequest"}},
		{"attach request truncated container", attachRequest[:10], []string{"AttachRequest"}},
		{"attach accept with default bearer request", attachAccept, []string{"AttachAccept", "ActivateDefaultEPSBearerContextRequest"}},
		{"attach complete", marshal(t, &eps.AttachComplete{ESMMessageContainer: defaultBearerAccept}), []string{"AttachComplete", "ActivateDefaultEPSBearerContextAccept"}},
		{"attach reject with esm container", marshal(t, &eps.AttachReject{Cause: eps.EMMCauseESMFailure, ESMMessageContainer: pdnConnectivityReject}), []string{"AttachReject", "PDNConnectivityReject"}},
		{"attach reject without esm container", marshal(t, &eps.AttachReject{Cause: eps.EMMCauseIllegalUE}), []string{"AttachReject"}},
		{"service reject", []byte{0x07, 0x4E, 0x09}, []string{"ServiceReject"}},
		{"emm status", []byte{0x07, 0x60, 0x6F}, []string{"EMMStatus"}},
		{"service request", []byte{0xC7, 0x01, 0x02, 0x03}, []string{"ServiceRequest"}},
		{"service request extension codepoint", []byte{0xD7, 0x01, 0x02, 0x03}, []string{"ServiceRequest"}},
		{"unassigned emm type", []byte{0x07, 0x40}, []string{NASTypeUnknown}},
		{"security protected", []byte{0x27, 0x00, 0x00, 0x00, 0x00, 0x01, 0x07, 0x4E}, []string{NASTypeUnknown}},
		{"modify bearer reject", []byte{0x62, 0x03, 0xCB, 0x1F}, []string{"ModifyEPSBearerContextReject"}},
		{"dedicated bearer accept", []byte{0x62, 0x00, 0xC6}, []string{"ActivateDedicatedEPSBearerContextAccept"}},
		{"unassigned esm type", []byte{0x02, 0x01, 0xC0}, []string{NASTypeUnknown}},
		{"unknown pd", []byte{0x05, 0x41}, []string{NASTypeUnknown}},
		{"emm too short", []byte{0x07}, []string{NASTypeUnknown}},
		{"empty", nil, []string{NASTypeUnknown}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nasTypes4G(tc.msg); !slices.Equal(got, tc.want) {
				t.Errorf("nasTypes4G = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadableEPS(t *testing.T) {
	inner := []byte{0x07, 0x48, 0x00}
	header := func(sht byte) []byte { return []byte{sht<<4 | 0x07, 0xAA, 0xBB, 0xCC, 0xDD, 0x03} }

	cases := []struct {
		name string
		pdu  []byte
		want []string
	}{
		{"plain", inner, []string{"TrackingAreaUpdateRequest"}},
		{"integrity protected", append(header(1), inner...), []string{"TrackingAreaUpdateRequest"}},
		{"integrity protected new context", append(header(3), 0x07, 0x5D), []string{"SecurityModeCommand"}},
		{"ciphered", append(header(2), inner...), []string{NASTypeUnknown}},
		{"ciphered new context", append(header(4), inner...), []string{NASTypeUnknown}},
		{"partially ciphered", append(header(5), inner...), []string{NASTypeUnknown}},
		{"integrity protected without body", header(1), []string{NASTypeUnknown}},
		{"service request", []byte{0xC7, 0x01, 0x02, 0x03}, []string{"ServiceRequest"}},
		{"standalone esm", []byte{0x02, 0x01, 0xD2}, []string{"PDNDisconnectRequest"}},
		{"empty", nil, []string{NASTypeUnknown}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nasTypes4G(readableEPS(tc.pdu)); !slices.Equal(got, tc.want) {
				t.Errorf("types = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNASTypeValuesAreBounded(t *testing.T) {
	sets := map[string][]string{RAT5G: {NASTypeUnknown}, RAT4G: {NASTypeUnknown, emmServiceRequest}}

	for _, name := range gmmTypes {
		sets[RAT5G] = append(sets[RAT5G], name)
	}

	for _, name := range gsmTypes {
		sets[RAT5G] = append(sets[RAT5G], name)
	}

	for _, name := range emmTypes {
		sets[RAT4G] = append(sets[RAT4G], name)
	}

	for _, name := range esmTypes {
		sets[RAT4G] = append(sets[RAT4G], name)
	}

	want := map[string]int{RAT5G: 1 + 37 + 20, RAT4G: 2 + 32 + 27}

	for rat, names := range sets {
		seen := map[string]bool{}

		for _, name := range names {
			if seen[name] {
				t.Errorf("rat %s: type %q is used by more than one message", rat, name)
			}

			seen[name] = true
		}

		if len(seen) != want[rat] {
			t.Errorf("rat %s: %d type values, want %d", rat, len(seen), want[rat])
		}
	}

	for code := range 256 {
		for _, got := range [][]string{
			nasTypes5G([]byte{0x7E, 0x00, byte(code), 0x00}),
			nasTypes5G([]byte{0x2E, 0x05, 0x01, byte(code)}),
		} {
			for _, name := range got {
				if !slices.Contains(sets[RAT5G], name) {
					t.Errorf("5G code %#x: type %q outside the bounded set", code, name)
				}
			}
		}

		for _, got := range [][]string{
			nasTypes4G([]byte{byte(code), byte(code), 0x00}),
			nasTypes4G([]byte{0x07, byte(code), 0x00}),
			nasTypes4G([]byte{0x02, 0x01, byte(code)}),
		} {
			for _, name := range got {
				if !slices.Contains(sets[RAT4G], name) {
					t.Errorf("4G code %#x: type %q outside the bounded set", code, name)
				}
			}
		}
	}
}

func TestNASMessageCounting(t *testing.T) {
	saved := nasMessages

	t.Cleanup(func() { nasMessages = saved })

	nasMessages = nil

	NASMessage5G(DirectionInbound, []byte{0x7E, 0x00, 0x4C})
	NASPDU4G(DirectionInbound, []byte{0xC7, 0x01, 0x02, 0x03})

	nasMessages = newNASMessages()

	reg := prometheus.NewRegistry()
	reg.MustRegister(nasMessages)

	ul := append([]byte{0x7E, 0x00, 0x67, 0x01, 0x00, byte(len(pduSessionEstablishmentRequest))}, pduSessionEstablishmentRequest...)

	NASMessage5G(DirectionInbound, ul)
	NASPDU5G(DirectionInbound, []byte{0x7E, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, 0x7E, 0x00, 0x4C})
	NASMessage5G(DirectionOutbound, []byte{0x7E, 0x00, 0x4D, 0x09})
	NASMessage4G(DirectionOutbound, []byte{0x07, 0x4E, 0x09})
	NASPDU4G(DirectionInbound, []byte{0xC7, 0x01, 0x02, 0x03})
	NASPDU4G(DirectionInbound, []byte{0x17, 0x00, 0x00, 0x00, 0x00, 0x01, 0x62, 0x00, 0xC6})

	want := map[[3]string]float64{
		{RAT5G, DirectionInbound, "ULNASTransport"}:                          1,
		{RAT5G, DirectionInbound, "PDUSessionEstablishmentRequest"}:          1,
		{RAT5G, DirectionInbound, NASTypeUnknown}:                            1,
		{RAT5G, DirectionOutbound, "ServiceReject"}:                          1,
		{RAT4G, DirectionOutbound, "ServiceReject"}:                          1,
		{RAT4G, DirectionInbound, "ServiceRequest"}:                          1,
		{RAT4G, DirectionInbound, "ActivateDedicatedEPSBearerContextAccept"}: 1,
	}

	got := map[[3]string]float64{}

	for _, m := range gatherFamily(t, reg, "app_nas_messages_total") {
		key := [3]string{labelValue(m.GetLabel(), "rat"), labelValue(m.GetLabel(), "direction"), labelValue(m.GetLabel(), "type")}
		got[key] = m.GetCounter().GetValue()
	}

	if !maps.Equal(got, want) {
		t.Errorf("app_nas_messages_total = %v, want %v", got, want)
	}
}
