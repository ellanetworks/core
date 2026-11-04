// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/s1ap"

type s1ConnKey struct {
	conn    S1APWriter
	enbUEID s1ap.ENBUES1APID
}

type s1ConnIndex struct {
	byKey   map[s1ConnKey]map[*UeConn]struct{}
	byAssoc map[S1APWriter]map[*UeConn]struct{}
}

func newS1ConnIndex() s1ConnIndex {
	return s1ConnIndex{
		byKey:   make(map[s1ConnKey]map[*UeConn]struct{}),
		byAssoc: make(map[S1APWriter]map[*UeConn]struct{}),
	}
}

func (x *s1ConnIndex) add(c *UeConn) {
	key := s1ConnKey{conn: c.Conn(), enbUEID: c.ENBUES1APID()}

	addToSet(x.byKey, key, c)
	addToSet(x.byAssoc, key.conn, c)

	c.indexedAt = &key
}

func (x *s1ConnIndex) remove(c *UeConn) {
	key := c.indexedAt
	if key == nil {
		return
	}

	removeFromSet(x.byKey, *key, c)
	removeFromSet(x.byAssoc, key.conn, c)

	c.indexedAt = nil
}

func (x *s1ConnIndex) matching(conn S1APWriter, enbUEID s1ap.ENBUES1APID) map[*UeConn]struct{} {
	return x.byKey[s1ConnKey{conn: conn, enbUEID: enbUEID}]
}

func (x *s1ConnIndex) onAssociation(conn S1APWriter) map[*UeConn]struct{} {
	return x.byAssoc[conn]
}

func addToSet[K comparable](m map[K]map[*UeConn]struct{}, k K, c *UeConn) {
	set, ok := m[k]
	if !ok {
		set = make(map[*UeConn]struct{})
		m[k] = set
	}

	set[c] = struct{}{}
}

func removeFromSet[K comparable](m map[K]map[*UeConn]struct{}, k K, c *UeConn) {
	set, ok := m[k]
	if !ok {
		return
	}

	delete(set, c)

	if len(set) == 0 {
		delete(m, k)
	}
}

func (m *MME) registerConnLocked(id uint32, c *UeConn) {
	m.conns[id] = c
	m.connIndex.add(c)
}

func (m *MME) rebindConnLocked(c *UeConn, w S1APWriter, enbUEID s1ap.ENBUES1APID) {
	registered := m.conns[uint32(c.MMEUES1APID)] == c
	if registered {
		m.connIndex.remove(c)
	}

	c.setConn(w)
	c.setENBUES1APID(enbUEID)

	if registered {
		m.connIndex.add(c)
	}
}

func (m *MME) setConnENBUES1APIDLocked(c *UeConn, enbUEID s1ap.ENBUES1APID) {
	m.rebindConnLocked(c, c.Conn(), enbUEID)
}
