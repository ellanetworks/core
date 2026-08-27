// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

// DownlinkBuffer is the downlink packet-buffering capability, satisfied by
// the BufferResponder in internal/upf.
type DownlinkBuffer interface {
	// Drain re-injects the session's buffered packets; called after the
	// FAR flips to FORW.
	Drain(seid uint64)

	// Drop discards the session's buffered packets; called when paging
	// fails or the session is deleted.
	Drop(seid uint64)
}

// SetDownlinkBuffer wires the buffering implementation into the engine.
// Nil disables buffering; every call site nil-checks.
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
