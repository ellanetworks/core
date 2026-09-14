// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package scenarios

import (
	"fmt"
	"time"
)

type ForwardingProbe struct {
	Send          func(payload []byte) error
	SendEndMarker func() error
	Received      func() int
	EndMarkers    func() int
}

const ForwardingProbePackets = 5

func DriveForwardingTunnel(p ForwardingProbe) error {
	for i := range ForwardingProbePackets {
		payload := []byte(fmt.Sprintf("indirect-forwarding-probe-%d", i))

		if err := p.Send(payload); err != nil {
			return fmt.Errorf("send probe %d on the forwarding tunnel: %w", i, err)
		}
	}

	if err := awaitCount(p.Received, ForwardingProbePackets, "user data relayed to the target's forwarding tunnel"); err != nil {
		return err
	}

	if err := p.SendEndMarker(); err != nil {
		return fmt.Errorf("send an End Marker on the forwarding tunnel: %w", err)
	}

	return awaitCount(p.EndMarkers, 1, "End Markers relayed to the target's forwarding tunnel")
}

func AwaitForwardingTunnelReleased(p ForwardingProbe) error {
	deadline := time.Now().Add(20 * time.Second)

	for time.Now().Before(deadline) {
		before := p.Received()

		if err := p.Send([]byte("indirect-forwarding-after-release")); err != nil {
			return fmt.Errorf("send on the released forwarding tunnel: %w", err)
		}

		time.Sleep(300 * time.Millisecond)

		if p.Received() == before {
			return nil
		}
	}

	return fmt.Errorf("the forwarding tunnel still relays after the handover completed")
}

func awaitCount(count func() int, want int, what string) error {
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		if got := count(); got >= want {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("%s = %d, want %d", what, count(), want)
}
