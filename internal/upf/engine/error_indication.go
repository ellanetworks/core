// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

const ohcGtpUMask uint8 = 0x03

type ErrorIndicationMatch struct {
	SEID  uint64
	FARID uint32
}

func (conn *SessionEngine) FindFARByRemoteFTEID(remote models.FTEID) (ErrorIndicationMatch, bool) {
	addr := ebpf.IPToIn6Addr(remote.Addr)

	var match ErrorIndicationMatch

	found := false

	conn.eachSession(func(session *Session) bool {
		farID, ok := session.findFAR(func(far ebpf.FarInfo) bool {
			return far.OuterHeaderCreation&ohcGtpUMask != 0 &&
				far.TeID == remote.TEID && far.RemoteIP == addr
		})
		if !ok {
			return true
		}

		match = ErrorIndicationMatch{SEID: session.SEID, FARID: farID}
		found = true

		return false
	})

	return match, found
}

func (conn *SessionEngine) SendErrorIndicationReport(ctx context.Context, smf SMFReportHandler, remote models.FTEID) error {
	match, ok := conn.FindFARByRemoteFTEID(remote)
	if !ok {
		return fmt.Errorf("no session forwards to remote F-TEID %#x at %s", remote.TEID, remote.Addr)
	}

	return smf.HandleErrorIndicationReport(ctx, &models.ErrorIndicationReport{
		SEID:        match.SEID,
		FARID:       match.FARID,
		RemoteFTEID: remote,
	})
}
