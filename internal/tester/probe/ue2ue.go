// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package probe

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	ue2uePayload    = "ue2ue-probe"
	ue2ueTimeout    = 3 * time.Second
	ue2ueRetryDelay = 200 * time.Millisecond
	ue2ueRetries    = 5
)

// UE2UE sends a UDP datagram from srcTun to dstIP:port and checks if it
// arrives on a listener bound to dstTun. Returns nil if the datagram was
// received, an error otherwise. Unlike ICMP ping, this is one-directional:
// the sender does not need a reverse route, only the forward path through
// the UPF must work.
func UE2UE(ctx context.Context, srcTun, dstTun, dstIP string, port int) error {
	lc := net.ListenConfig{Control: bindToDeviceControl(dstTun)}

	pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("listen on %s: %w", dstTun, err)
	}

	defer func() { _ = pc.Close() }()

	received := make(chan struct{}, 1)

	go func() {
		buf := make([]byte, 256)

		_ = pc.SetReadDeadline(time.Now().Add(ue2ueTimeout))
		if _, _, err := pc.ReadFrom(buf); err == nil {
			select {
			case received <- struct{}{}:
			default:
			}
		}
	}()

	dialer := net.Dialer{
		Control: bindToDeviceControl(srcTun),
	}

	for range ue2ueRetries {
		conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(dstIP, strconv.Itoa(port)))
		if err != nil {
			time.Sleep(ue2ueRetryDelay)
			continue
		}

		_, _ = conn.Write([]byte(ue2uePayload))
		_ = conn.Close()

		select {
		case <-received:
			return nil
		case <-time.After(ue2ueRetryDelay):
		}
	}

	select {
	case <-received:
		return nil
	case <-time.After(ue2ueTimeout):
		return fmt.Errorf("udp %s -> %s:%d: datagram not received on %s", srcTun, dstIP, port, dstTun)
	case <-ctx.Done():
		return ctx.Err()
	}
}
