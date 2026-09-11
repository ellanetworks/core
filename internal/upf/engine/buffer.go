// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

// DownlinkBuffer is the downlink packet-buffering capability.
type DownlinkBuffer interface {
	// Drain re-injects the session's buffered packets.
	Drain(seid uint64)

	// Drop discards the session's buffered packets.
	Drop(seid uint64)
}

// SetDownlinkBuffer wires the buffering implementation into the engine.
func (pc *SessionEngine) SetDownlinkBuffer(b DownlinkBuffer) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.dlBuffer = b
}

func (pc *SessionEngine) downlinkBuffer() DownlinkBuffer {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.dlBuffer
}
