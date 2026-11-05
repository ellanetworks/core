// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "github.com/ellanetworks/core/internal/models"

type ranConnKey struct {
	conn NGAPWriter
	id   models.RanUeNgapID
}

func (a *AMF) indexConnLocked(ueConn *UeConn) {
	if a.connsByRan == nil {
		a.connsByRan = make(map[ranConnKey]map[*UeConn]struct{})
	}

	k := ranConnKey{conn: ueConn.Conn(), id: ueConn.RanUeNgapID()}

	set := a.connsByRan[k]
	if set == nil {
		set = make(map[*UeConn]struct{}, 1)
		a.connsByRan[k] = set
	}

	set[ueConn] = struct{}{}
}

func (a *AMF) unindexConnLocked(ueConn *UeConn) {
	k := ranConnKey{conn: ueConn.Conn(), id: ueConn.RanUeNgapID()}

	set := a.connsByRan[k]
	if set == nil {
		return
	}

	delete(set, ueConn)

	if len(set) == 0 {
		delete(a.connsByRan, k)
	}
}

func (a *AMF) connsByRanLocked(conn NGAPWriter, id models.RanUeNgapID) []*UeConn {
	set := a.connsByRan[ranConnKey{conn: conn, id: id}]
	if len(set) == 0 {
		return nil
	}

	out := make([]*UeConn, 0, len(set))

	for ueConn := range set {
		if ueConn.Conn() == conn && ueConn.RanUeNgapID() == id && a.conns[int64(ueConn.AmfUeNgapID)] == ueConn {
			out = append(out, ueConn)
		}
	}

	return out
}
