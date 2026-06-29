// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

// EMMState returns the UE's EMM registration state.
func (ue *UeContext) EMMState() EMMState { return ue.emmState.load() }

// SetEMMState sets the UE's EMM registration state.
func (ue *UeContext) SetEMMState(s EMMState) { ue.emmState.store(s) }

// ResyncTried reports whether SQN re-synchronisation has already been attempted
// for the in-progress authentication (TS 33.401).
func (ue *UeContext) ResyncTried() bool { return ue.resyncTried }

// SetResyncTried records whether SQN re-synchronisation has been attempted.
func (ue *UeContext) SetResyncTried(v bool) { ue.resyncTried = v }

// PDNCount returns the number of the UE's PDN connections, read under the lock.
func (ue *UeContext) PDNCount() int {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.Pdns)
}

// CommitBearerModification commits a PDN connection's pending in-place
// modification under the lock, reporting false (a no-op) if no modification was
// in flight (TS 24.301 §6.4.2.3).
func (ue *UeContext) CommitBearerModification(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !p.Modifying {
		return false
	}

	p.DnConfig = p.PendingDNConfig
	p.SessAmbrDLBps = p.PendingSessAmbrDLBps
	p.SessAmbrULBps = p.PendingSessAmbrULBps
	p.Qci = p.PendingQCI
	p.Arp = p.PendingARP
	ClearPendingModifyLocked(p)

	return true
}

// ClearPendingModify clears a PDN connection's in-flight modification
// bookkeeping under the lock (TS 24.301 §6.4.2.4).
func (ue *UeContext) ClearPendingModify(p *PdnConnection) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ClearPendingModifyLocked(p)
}

// BearerReleaseOnly reports whether deactivating p releases only that PDN
// connection (an additional PDN, or a disconnect) rather than detaching the UE
// (TS 24.301 §6.4.4.2/§6.5.2).
func (ue *UeContext) BearerReleaseOnly(p *PdnConnection) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return p.Ebi != ue.DefaultEBI || p.Disconnecting
}

// ClearDeactivating clears a PDN connection's in-flight deactivation flag under
// the lock.
func (ue *UeContext) ClearDeactivating(p *PdnConnection) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	p.Deactivating = false
}

// SecureExchangeEstablished reports whether secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *S1Conn) SecureExchangeEstablished() bool {
	if c == nil {
		return false
	}

	return c.secureExchangeEstablished
}

// MarkSecureExchangeEstablished records that secure exchange of NAS messages is
// established on the connection (TS 24.301 §4.4.4.3).
func (c *S1Conn) MarkSecureExchangeEstablished() {
	if c != nil {
		c.secureExchangeEstablished = true
	}
}
