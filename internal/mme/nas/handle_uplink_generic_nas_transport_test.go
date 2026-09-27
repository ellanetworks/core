// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

type recordingLPPHandler struct {
	calls         int
	supi          etsi.SUPI
	correlationID []byte
	lppData       []byte
}

func (h *recordingLPPHandler) ForwardLPP(_ context.Context, supi etsi.SUPI, correlationID, lppData []byte) error {
	h.calls++
	h.supi = supi
	h.correlationID = correlationID
	h.lppData = lppData

	return nil
}

func uplinkGenericNASTransport(t *testing.T, containerType eps.GenericMessageContainerType) []byte {
	t.Helper()

	plain, err := (&eps.UplinkGenericNASTransport{
		ContainerType:         containerType,
		Container:             []byte{0x92, 0x00},
		AdditionalInformation: []byte{0x00, 0x00, 0x00, 0x05},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return plain
}

func TestUplinkGenericNASTransport_ForwardsLPPToLMF(t *testing.T) {
	m := newTestMME(t)
	h := &recordingLPPHandler{}
	m.LPPHandler = h

	ue, cc := securedUE(t, m)

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), uplinkGenericNASTransport(t, eps.GenericMessageContainerTypeLPP), true)

	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want handled", d)
	}

	if h.calls != 1 {
		t.Fatalf("ForwardLPP calls = %d, want 1", h.calls)
	}

	if h.supi != ue.Supi() {
		t.Errorf("SUPI = %s, want %s", h.supi, ue.Supi())
	}

	if !bytes.Equal(h.correlationID, []byte{0x00, 0x00, 0x00, 0x05}) {
		t.Errorf("correlation id = %x", h.correlationID)
	}

	if !bytes.Equal(h.lppData, []byte{0x92, 0x00}) {
		t.Errorf("LPP data = %x", h.lppData)
	}

	if cc.count() != 0 {
		t.Errorf("sent = %d downlink(s), want none", cc.count())
	}
}

func TestUplinkGenericNASTransport_IgnoresOtherContainerTypes(t *testing.T) {
	m := newTestMME(t)
	h := &recordingLPPHandler{}
	m.LPPHandler = h

	ue, _ := securedUE(t, m)

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), uplinkGenericNASTransport(t, eps.GenericMessageContainerTypeLocationServices), true)

	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want handled", d)
	}

	if h.calls != 0 {
		t.Errorf("ForwardLPP calls = %d, want 0", h.calls)
	}
}

func TestUplinkGenericNASTransport_OutsideRegisteredIsSilent(t *testing.T) {
	m := newTestMME(t)
	h := &recordingLPPHandler{}
	m.LPPHandler = h

	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMDeregistered)

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), uplinkGenericNASTransport(t, eps.GenericMessageContainerTypeLPP), true)

	if d.Action != nasreply.ActionSilent {
		t.Fatalf("disposition = %+v, want silent", d)
	}

	if h.calls != 0 {
		t.Errorf("ForwardLPP calls = %d, want 0", h.calls)
	}
}
