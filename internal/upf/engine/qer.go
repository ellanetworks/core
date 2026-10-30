// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var _ = [1]struct{}{}[ebpf.MaxSessionQERs-models.MaxSessionQERs]

func checkSessionQERCount(session *Session) error {
	if n := len(session.ListQERs()); n > ebpf.MaxSessionQERs {
		return fmt.Errorf("the session needs %d QERs, the user plane holds %d", n, ebpf.MaxSessionQERs)
	}

	return nil
}

func checkSessionQERReferences(session *Session) error {
	if err := checkSessionQERCount(session); err != nil {
		return err
	}

	qers := session.ListQERs()

	for _, pdr := range session.ListPDRs() {
		id := pdr.PdrInfo.QerID
		if _, ok := qers[id]; id != 0 && !ok {
			return fmt.Errorf("PDR %d references QER %d, which the session does not keep", pdr.PdrID, id)
		}
	}

	return nil
}

func (conn *SessionEngine) createQERMeters(session *Session, existing map[uint32]ebpf.QerInfo) ([]uint32, error) {
	var created []uint32

	for _, id := range slices.Sorted(maps.Keys(session.ListQERs())) {
		if _, ok := existing[id]; ok {
			continue
		}

		if _, owned := session.qerMeters[id]; owned {
			return created, fmt.Errorf("QER %d still owns a meter awaiting cleanup", id)
		}

		if conn.BpfObjects != nil {
			if conn.BpfObjects.QerWindows == nil {
				return created, fmt.Errorf("QER meter map is not initialized")
			}

			if err := conn.BpfObjects.CreateQerMeter(session.SEID, id); err != nil {
				return created, err
			}
		}

		if session.qerMeters == nil {
			session.qerMeters = make(map[uint32]struct{})
		}

		session.qerMeters[id] = struct{}{}
		created = append(created, id)
	}

	return created, nil
}

func (conn *SessionEngine) deleteQERMeters(session *Session, ids []uint32) error {
	var errs []error

	for _, id := range ids {
		if _, owned := session.qerMeters[id]; !owned {
			continue
		}

		if conn.BpfObjects != nil {
			if conn.BpfObjects.QerWindows == nil {
				errs = append(errs, fmt.Errorf("QER meter map is not initialized"))
				break
			}

			if err := conn.BpfObjects.DeleteQerMeter(session.SEID, id); err != nil {
				errs = append(errs, err)
				continue
			}
		}

		delete(session.qerMeters, id)
	}

	return errors.Join(errs...)
}

func retiredQERMeters(session *Session) []uint32 {
	var retired []uint32

	active := session.ListQERs()
	for id := range session.qerMeters {
		if _, ok := active[id]; !ok {
			retired = append(retired, id)
		}
	}

	slices.Sort(retired)

	return retired
}

func (conn *SessionEngine) retireQERMeters(session *Session) error {
	retired := retiredQERMeters(session)
	if len(retired) == 0 {
		return nil
	}

	if !session.unpublished {
		if err := conn.waitForPrograms(session); err != nil {
			return err
		}
	}

	return conn.deleteQERMeters(session, retired)
}
