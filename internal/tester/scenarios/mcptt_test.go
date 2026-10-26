// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/ims/sip"
	"github.com/ellanetworks/ims/sip/sdp"
)

func TestMCPTTOfferAnswer(t *testing.T) {
	offer := mcpttMedia{
		addr: netip.MustParseAddr("10.62.0.1"), id: 7, version: 1, audio: 50000, control: 50002,
		pt: "96", rtpmap: "AMR-WB/16000/1", fmtp: "mode-change-capability=2;max-red=0", floorSSRC: 1001,
	}.sdp()

	answer, err := mcpttAnswer(netip.MustParseAddr("10.3.0.3"), offer)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}

	s, err := sdp.Parse(answer.sdp())
	if err != nil {
		t.Fatalf("parse the answer: %v", err)
	}

	if len(s.Media) != 2 {
		t.Fatalf("answer with %d media, want 2", len(s.Media))
	}

	audio, _ := s.Media[0].Desc()
	control, _ := s.Media[1].Desc()

	switch {
	case audio.Type != sdp.Audio || audio.Port != mcpttServerAudioPort || len(audio.Formats) != 1 || audio.Formats[0] != "96":
		t.Errorf("answer audio %+v", audio)
	case control.Type != "application" || control.Proto != "udp" || len(control.Formats) != 1 || control.Formats[0] != "MCPTT":
		t.Errorf("answer media plane control channel %+v", control)
	}

	if f, ok := s.Media[1].Fmtp("MCPTT"); !ok || f != "mc_floor_ssrc=2002" {
		t.Errorf("answer fmtp:MCPTT %q", f)
	}

	if maps, err := s.Media[0].RTPMaps(); err != nil || len(maps) != 1 || maps[0].Encoding != "AMR-WB" || maps[0].ClockRate != 16000 {
		t.Errorf("answer rtpmap %+v, %v", maps, err)
	}
}

func TestMCPTTAnswerRefusesAudioOnly(t *testing.T) {
	offer := "v=0\r\no=- 1 1 IN IP4 10.62.0.1\r\ns=-\r\nc=IN IP4 10.62.0.1\r\nt=0 0\r\nm=audio 50000 RTP/AVP 96\r\n"

	if _, err := mcpttAnswer(netip.MustParseAddr("10.3.0.3"), []byte(offer)); err == nil {
		t.Fatal("an offer without the media plane control channel was answered")
	}
}

func TestMCPTTServerResourcePriority(t *testing.T) {
	req := sip.NewRequest("INVITE", sip.URI{Scheme: "sip", User: "grp-1", Host: MCPTTPSIDomain})
	req.Header.Add("Resource-Priority", "mcpttp.11")

	s := &MCPTTServer{changed: make(chan struct{})}

	if got := s.responseHeader(req).Get("Resource-Priority"); got != "mcpttp.11" {
		t.Errorf("echoed Resource-Priority %q, want mcpttp.11", got)
	}

	s.SetResourcePriority("mcpttp.13")

	h := s.responseHeader(req)
	if got := h.Get("Resource-Priority"); got != "mcpttp.13" {
		t.Errorf("corrected Resource-Priority %q, want mcpttp.13", got)
	}

	caps, err := h.FeatureCaps()
	if err != nil || len(caps) != 1 || len(caps[0].Tokens("g.3gpp.icsi-ref")) != 1 || caps[0].Tokens("g.3gpp.icsi-ref")[0] != MCPTTICSI {
		t.Errorf("Feature-Caps %q: %+v, %v", h.Values("Feature-Caps"), caps, err)
	}
}
